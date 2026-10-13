package live

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sbondCo/Watcharr/config"
	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	wimage "github.com/sbondCo/Watcharr/image"
)

func pngOf(c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 8, 12))
	for x := 0; x < 8; x++ {
		for y := 0; y < 12; y++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// posterServer serves two distinct posters, a fake one, and counts hits.
func posterServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	red, blue := pngOf(color.RGBA{255, 0, 0, 255}), pngOf(color.RGBA{0, 0, 255, 255})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/red.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(red)
		case "/blue.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(blue)
		case "/fake.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("<html>not an image</html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func countRows(t *testing.T, s *Service, model any) int64 {
	t.Helper()
	var n int64
	if err := s.db.Model(model).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func showReq(title, poster string) domain.LiveShowRequest {
	return domain.LiveShowRequest{Title: title, Date: "2026-11-05T20:00:00Z", PosterURL: poster}
}

func wantPosterError(t *testing.T, err error, reason string) {
	t.Helper()
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("want a *ValidationError, got %v", err)
	}
	if !strings.HasPrefix(verr.Msg, "could not use that poster image: ") ||
		!strings.Contains(verr.Msg, reason) {
		t.Errorf("message = %q, want the poster prefix and %q", verr.Msg, reason)
	}
}

func TestPosterLifecycle(t *testing.T) {
	s := newTestService(t)
	srv, hits := posterServer(t)

	// Create: downloaded, re-encoded, stored under img/live/.
	created, err := s.Create(1, showReq("A", srv.URL+"/red.png"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.PosterURL != srv.URL+"/red.png" || created.Poster == nil {
		t.Fatalf("poster not attached: %+v", created)
	}
	first := *created.Poster
	if !strings.HasPrefix(first.Path, "img/live/") || first.BlurHash == "" || first.CreatedAt.IsZero() {
		t.Errorf("unexpected image: %+v", first)
	}
	if _, err := os.Stat(path.Join(config.DataPath, first.Path)); err != nil {
		t.Errorf("image file missing: %v", err)
	}
	if n := countRows(t, s, &entity.Image{}); n != 1 {
		t.Errorf("images = %d, want 1", n)
	}

	// Get and List load it.
	got, _ := s.Get(1, created.ID)
	if got.Poster == nil || got.Poster.Path != first.Path {
		t.Errorf("Get: poster not loaded: %+v", got.Poster)
	}
	list, _ := s.List(1)
	if len(list) != 1 || list[0].Poster == nil || list[0].Poster.Path != first.Path {
		t.Errorf("List: poster not loaded: %+v", list)
	}

	// Same URL: kept, not downloaded again.
	before := hits.Load()
	upd, err := s.Update(1, created.ID, showReq("A2", srv.URL+"/red.png"))
	if err != nil {
		t.Fatalf("Update same URL: %v", err)
	}
	if hits.Load() != before {
		t.Error("same poster URL was downloaded again")
	}
	if upd.Poster == nil || upd.Poster.Path != first.Path || upd.Title != "A2" {
		t.Errorf("same URL: %+v", upd)
	}
	if n := countRows(t, s, &entity.Image{}); n != 1 {
		t.Errorf("images = %d, want 1", n)
	}

	// New URL: new poster.
	upd, err = s.Update(1, created.ID, showReq("A2", srv.URL+"/blue.png"))
	if err != nil {
		t.Fatalf("Update new URL: %v", err)
	}
	if upd.Poster == nil || upd.Poster.Path == first.Path || upd.PosterURL != srv.URL+"/blue.png" {
		t.Errorf("new URL: %+v", upd)
	}
	if n := countRows(t, s, &entity.Image{}); n != 2 {
		t.Errorf("images = %d, want 2", n)
	}

	// Empty: reference removed, image row and file kept.
	upd, err = s.Update(1, created.ID, showReq("A2", ""))
	if err != nil {
		t.Fatalf("Update empty: %v", err)
	}
	if upd.Poster != nil || upd.PosterURL != "" {
		t.Errorf("poster not removed: %+v", upd)
	}
	got, _ = s.Get(1, created.ID)
	if got.Poster != nil || got.PosterURL != "" {
		t.Errorf("poster removal not persisted: %+v", got)
	}
	if n := countRows(t, s, &entity.Image{}); n != 2 {
		t.Errorf("images = %d, want 2 (rows are not deleted)", n)
	}
}

func TestPosterRejected(t *testing.T) {
	s := newTestService(t)
	srv, _ := posterServer(t)

	for _, tc := range []struct{ poster, reason string }{
		{srv.URL + "/fake.png", "not a JPEG, PNG or GIF image"},
		{srv.URL + "/missing.png", "download failed"},
	} {
		_, err := s.Create(1, showReq("A", tc.poster))
		wantPosterError(t, err, tc.reason)
	}
	if n := countRows(t, s, &entity.LiveShow{}); n != 0 {
		t.Errorf("a show was created despite the bad poster (%d)", n)
	}
	if n := countRows(t, s, &entity.Image{}); n != 0 {
		t.Errorf("an image row was created (%d)", n)
	}

	// Update: nothing changes, the existing poster included.
	created, err := s.Create(1, showReq("A", srv.URL+"/red.png"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = s.Update(1, created.ID, showReq("B", srv.URL+"/fake.png"))
	wantPosterError(t, err, "not a JPEG, PNG or GIF image")
	got, _ := s.Get(1, created.ID)
	if got.Title != "A" || got.PosterURL != srv.URL+"/red.png" || got.Poster == nil ||
		got.Poster.Path != created.Poster.Path {
		t.Errorf("show altered by a failed update: %+v", got)
	}
}

// With the production fetcher, a poster on the LAN is refused at dial time.
func TestPosterBlocksLocalAddresses(t *testing.T) {
	s := newTestService(t)
	s.fetch = buildFetcher(false, true)
	srv, hits := posterServer(t)
	_, err := s.Create(1, showReq("A", srv.URL+"/red.png"))
	wantPosterError(t, err, "address not allowed")
	if hits.Load() != 0 {
		t.Error("the local server was reached")
	}

	s.fetch = newFetcher()
	_, err = s.Create(1, showReq("A", srv.URL+"/red.png"))
	wantPosterError(t, err, "port")
}

// The daily image cleanup only knew about avatars: it must leave posters of
// live shows alone, soft-deleted ones included.
func TestImageCleanupKeepsPosters(t *testing.T) {
	s := newTestService(t)
	srv, _ := posterServer(t)
	a, err := s.Create(1, showReq("A", srv.URL+"/red.png"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Create(1, showReq("B", srv.URL+"/blue.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(1, b.ID); err != nil {
		t.Fatal(err)
	}
	wimage.CleanupImages(s.db)
	if n := countRows(t, s, &entity.Image{}); n != 2 {
		t.Errorf("images = %d after cleanup, want 2", n)
	}
	if _, err := os.Stat(path.Join(config.DataPath, a.Poster.Path)); err != nil {
		t.Errorf("poster file removed: %v", err)
	}

	// Unreferenced (poster cleared): fair game.
	if _, err := s.Update(1, a.ID, showReq("A", "")); err != nil {
		t.Fatal(err)
	}
	wimage.CleanupImages(s.db)
	if n := countRows(t, s, &entity.Image{}); n != 1 {
		t.Errorf("images = %d after cleanup, want 1", n)
	}
}

func TestPosterHTTPContract(t *testing.T) {
	e := testEngine(t)
	srv, _ := posterServer(t)

	w := do(e, "POST", "/api/live/shows",
		`{"title":"A","date":"2026-11-05T20:00:00Z","posterUrl":"`+srv.URL+`/red.png"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var obj struct {
		PosterURL string         `json:"posterUrl"`
		Poster    map[string]any `json:"poster"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &obj); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range obj.Poster {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if obj.PosterURL != srv.URL+"/red.png" || strings.Join(keys, ",") != "blurHash,createdAt,path" {
		t.Errorf("posterUrl=%q poster keys=%v", obj.PosterURL, keys)
	}

	w = do(e, "POST", "/api/live/shows",
		`{"title":"A","date":"2026-11-05T20:00:00Z","posterUrl":"`+srv.URL+`/fake.png"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "could not use that poster image") {
		t.Errorf("bad poster: %d %s", w.Code, w.Body)
	}
}
