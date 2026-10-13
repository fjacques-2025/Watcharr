package live

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sbondCo/Watcharr/config"
	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	wimage "github.com/sbondCo/Watcharr/image"
)

var (
	red   = color.RGBA{255, 0, 0, 255}
	blue  = color.RGBA{0, 0, 255, 255}
	green = color.RGBA{0, 255, 0, 255}
)

// upload posts a multipart form; file nil leaves the `photo` part out.
func upload(e *gin.Engine, showId uint, file []byte, fields map[string]string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if file != nil {
		fw, _ := mw.CreateFormFile("photo", "photo.png")
		fw.Write(file)
	}
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	mw.Close()
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/live/shows/%d/photos", showId), &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

func mustUpload(t *testing.T, e *gin.Engine, showId uint, file []byte, fields map[string]string) domain.LivePhoto {
	t.Helper()
	w := upload(e, showId, file, fields)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	var p domain.LivePhoto
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func photoOrder(ps []domain.LivePhoto) string {
	var n []string
	for _, p := range ps {
		n = append(n, fmt.Sprintf("%s/%v", p.Caption, p.IsTicket))
	}
	return strings.Join(n, ",")
}

func sortedKeys(m map[string]any) string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func TestPhotoUploadAndOrder(t *testing.T) {
	s := newTestService(t)
	e := testEngineWith(s)
	show := mustCreate(t, s, 1, "A", "2026-10-01T20:00:00Z")

	w := upload(e, show.ID, pngOf(red), map[string]string{"caption": "r"})
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	var obj map[string]any
	json.Unmarshal(w.Body.Bytes(), &obj)
	img, _ := obj["image"].(map[string]any)
	if sortedKeys(obj) != "caption,id,image,isTicket" || sortedKeys(img) != "blurHash,createdAt,path" ||
		obj["isTicket"] != false || obj["caption"] != "r" {
		t.Fatalf("photo JSON: %s", w.Body)
	}
	p := img["path"].(string)
	if !strings.HasPrefix(p, "img/live-photos/") || !strings.HasSuffix(p, ".jpg") || img["blurHash"] == "" {
		t.Errorf("image: %v", img)
	}
	if _, err := os.Stat(path.Join(config.DataPath, p)); err != nil {
		t.Errorf("file missing: %v", err)
	}

	ticket := mustUpload(t, e, show.ID, pngOf(blue), map[string]string{"isTicket": "true", "caption": "  b  "})
	if !ticket.IsTicket || ticket.Caption != "b" {
		t.Errorf("ticket: %+v", ticket)
	}
	g := mustUpload(t, e, show.ID, pngOf(green), map[string]string{"isTicket": "false", "caption": "g"})

	got, _ := s.Get(1, show.ID)
	if o := photoOrder(got.Photos); o != "b/true,r/false,g/false" {
		t.Errorf("order = %s", o)
	}
	if got.Photos[0].Image.Path == "" || got.Photos[0].Image.CreatedAt.IsZero() {
		t.Errorf("image not loaded: %+v", got.Photos[0])
	}

	// PUT: green becomes a ticket, after the first one by upload order.
	w = do(e, "PUT", fmt.Sprintf("/api/live/shows/%d/photos/%d", show.ID, g.ID), `{"isTicket":true,"caption":" G "}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"caption":"G"`) || !strings.Contains(w.Body.String(), `"path":"img/live-photos/`) {
		t.Errorf("PUT: %d %s", w.Code, w.Body)
	}
	list, _ := s.List(1)
	if o := photoOrder(list[0].Photos); o != "b/true,G/true,r/false" {
		t.Errorf("order after PUT = %s", o)
	}
	// Full replace: absent fields reset.
	do(e, "PUT", fmt.Sprintf("/api/live/shows/%d/photos/%d", show.ID, ticket.ID), `{}`)
	got, _ = s.Get(1, show.ID)
	if o := photoOrder(got.Photos); o != "G/true,r/false,/false" {
		t.Errorf("order after reset = %s", o)
	}
}

func TestPhotoRejects(t *testing.T) {
	s := newTestService(t)
	e := testEngineWith(s)
	show := mustCreate(t, s, 1, "A", "2026-10-01T20:00:00Z")

	for _, tc := range []struct {
		file   []byte
		fields map[string]string
		code   int
		msg    string
	}{
		{[]byte("<html>not an image</html>"), nil, 400, "could not use that photo: not a JPEG, PNG or GIF image"},
		{pngOf(red)[:60], nil, 400, "could not use that photo: the image is corrupt"},
		{nil, nil, 400, "photo is required"},
		{pngOf(red), map[string]string{"isTicket": "maybe"}, 400, "isTicket must be true or false"},
		{pngOf(red), map[string]string{"caption": strings.Repeat("é", 301)}, 400, "caption must be at most 300 characters"},
		// Over the saver's 10 MiB but under the body cap.
		{bytes.Repeat([]byte{0}, 11<<20), nil, 400, "could not use that photo: file is too big"},
		// Over the body cap: refused while parsing.
		{bytes.Repeat([]byte{0}, 13<<20), nil, 413, "could not use that photo: file is too big"},
	} {
		w := upload(e, show.ID, tc.file, tc.fields)
		if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.msg) {
			t.Errorf("got %d %s, want %d %q", w.Code, w.Body, tc.code, tc.msg)
		}
	}
	if w := upload(e, 999, pngOf(red), nil); w.Code != 404 {
		t.Errorf("missing show: %d", w.Code)
	}
	if n := countRows(t, s, &entity.Image{}); n != 0 {
		t.Errorf("images = %d, want 0", n)
	}
	if n := countRows(t, s, &entity.LivePhoto{}); n != 0 {
		t.Errorf("photos = %d, want 0", n)
	}
	if entries, _ := os.ReadDir(path.Join(config.DataPath, "img")); len(entries) != 0 {
		t.Errorf("files written: %v", entries)
	}
	if w := do(e, "PUT", fmt.Sprintf("/api/live/shows/%d/photos/1", show.ID), `{"caption":"x"}`); w.Code != 404 {
		t.Errorf("PUT missing photo: %d", w.Code)
	}
	if w := do(e, "DELETE", fmt.Sprintf("/api/live/shows/%d/photos/abc", show.ID), ""); w.Code != 400 {
		t.Errorf("DELETE bad id: %d", w.Code)
	}
}

func TestPhotoLimit(t *testing.T) {
	s := newTestService(t)
	e := testEngineWith(s)
	show := mustCreate(t, s, 1, "A", "2026-10-01T20:00:00Z")
	first := mustUpload(t, e, show.ID, pngOf(red), nil)
	// 49 more rows sharing the image: same as uploads, much faster.
	var img entity.Image
	s.db.Where("path = ?", first.Image.Path).Take(&img)
	rows := make([]entity.LivePhoto, 49)
	for i := range rows {
		rows[i] = entity.LivePhoto{ShowID: show.ID, ImageID: img.ID, Ord: i + 2}
	}
	if err := s.db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	w := upload(e, show.ID, pngOf(blue), nil)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "at most 50 photos per show") {
		t.Errorf("51st photo: %d %s", w.Code, w.Body)
	}
	if n := countRows(t, s, &entity.Image{}); n != 1 {
		t.Errorf("images = %d, want 1", n)
	}
	// Deleting frees a slot; ord continues from the max.
	if w := do(e, "DELETE", fmt.Sprintf("/api/live/shows/%d/photos/%d", show.ID, first.ID), ""); w.Code != 204 {
		t.Fatalf("delete: %d", w.Code)
	}
	p := mustUpload(t, e, show.ID, pngOf(blue), nil)
	var stored entity.LivePhoto
	s.db.Take(&stored, p.ID)
	if stored.Ord != 51 {
		t.Errorf("ord = %d, want 51", stored.Ord)
	}
}

func TestPhotoScoping(t *testing.T) {
	s := newTestService(t)
	e1, e2 := testEngineAs(s, 1), testEngineAs(s, 2)
	a := mustCreate(t, s, 1, "A", "2026-10-01T20:00:00Z")
	b := mustCreate(t, s, 1, "B", "2026-10-02T20:00:00Z")
	p := mustUpload(t, e1, a.ID, pngOf(red), map[string]string{"caption": "mine"})

	if w := upload(e2, a.ID, pngOf(blue), nil); w.Code != 404 {
		t.Errorf("upload to another user's show: %d", w.Code)
	}
	photo := fmt.Sprintf("/api/live/shows/%d/photos/%d", a.ID, p.ID)
	wrongShow := fmt.Sprintf("/api/live/shows/%d/photos/%d", b.ID, p.ID)
	for _, tc := range []struct {
		e            *gin.Engine
		method, path string
	}{
		{e2, "PUT", photo}, {e2, "DELETE", photo},
		{e1, "PUT", wrongShow}, {e1, "DELETE", wrongShow},
	} {
		if w := do(tc.e, tc.method, tc.path, `{"isTicket":true,"caption":"x"}`); w.Code != 404 {
			t.Errorf("%s %s: %d", tc.method, tc.path, w.Code)
		}
	}
	got, _ := s.Get(1, a.ID)
	if photoOrder(got.Photos) != "mine/false" {
		t.Errorf("photo altered: %s", photoOrder(got.Photos))
	}
	if w := do(e1, "DELETE", photo, ""); w.Code != 204 {
		t.Errorf("delete: %d", w.Code)
	}
	if w := do(e1, "DELETE", photo, ""); w.Code != 404 {
		t.Errorf("delete again: %d", w.Code)
	}
	if got, _ = s.Get(1, a.ID); len(got.Photos) != 0 {
		t.Errorf("photo still listed: %+v", got.Photos)
	}

	// A deleted show's photos are out of reach.
	q := mustUpload(t, e1, b.ID, pngOf(blue), nil)
	s.Delete(1, b.ID)
	if w := do(e1, "PUT", fmt.Sprintf("/api/live/shows/%d/photos/%d", b.ID, q.ID), `{}`); w.Code != 404 {
		t.Errorf("photo of deleted show: %d", w.Code)
	}
	if w := upload(e1, b.ID, pngOf(red), nil); w.Code != 404 {
		t.Errorf("upload to deleted show: %d", w.Code)
	}
}

// The nightly image cleanup must keep photos, those of soft-deleted shows
// included, and free them once their row is deleted.
func TestImageCleanupKeepsPhotos(t *testing.T) {
	s := newTestService(t)
	e := testEngineWith(s)
	a := mustCreate(t, s, 1, "A", "2026-10-01T20:00:00Z")
	b := mustCreate(t, s, 1, "B", "2026-10-02T20:00:00Z")
	kept := mustUpload(t, e, a.ID, pngOf(red), nil)
	gone := mustUpload(t, e, a.ID, pngOf(blue), nil)
	mustUpload(t, e, b.ID, pngOf(green), nil)
	s.Delete(1, b.ID)

	wimage.CleanupImages(s.db)
	if n := countRows(t, s, &entity.Image{}); n != 3 {
		t.Errorf("images = %d after cleanup, want 3", n)
	}

	if w := do(e, "DELETE", fmt.Sprintf("/api/live/shows/%d/photos/%d", a.ID, gone.ID), ""); w.Code != 204 {
		t.Fatalf("delete: %d", w.Code)
	}
	wimage.CleanupImages(s.db)
	if n := countRows(t, s, &entity.Image{}); n != 2 {
		t.Errorf("images = %d after cleanup, want 2", n)
	}
	if _, err := os.Stat(path.Join(config.DataPath, gone.Image.Path)); !os.IsNotExist(err) {
		t.Errorf("deleted photo's file still there: %v", err)
	}
	if _, err := os.Stat(path.Join(config.DataPath, kept.Image.Path)); err != nil {
		t.Errorf("kept photo's file removed: %v", err)
	}
}
