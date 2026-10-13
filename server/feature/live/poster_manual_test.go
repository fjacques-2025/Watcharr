package live

import (
	"bytes"
	"encoding/json"
	"image/color"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
)

func uploadPoster(e *gin.Engine, showId uint, file []byte) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if file != nil {
		fw, _ := mw.CreateFormFile("poster", "poster.jpg")
		fw.Write(file)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/live/shows/"+uintStr(showId)+"/poster", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

func uintStr(n uint) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func posterOf(t *testing.T, w *httptest.ResponseRecorder) (url string, path string) {
	t.Helper()
	var show domain.LiveShow
	if err := json.Unmarshal(w.Body.Bytes(), &show); err != nil {
		t.Fatalf("bad JSON: %s", w.Body)
	}
	if show.Poster != nil {
		path = show.Poster.Path
	}
	return show.PosterURL, path
}

// An uploaded poster has no source URL; saving the form (which then sends
// posterUrl "") must keep it, and only RemovePoster drops it.
func TestUploadedPosterSurvivesFormSave(t *testing.T) {
	s := newTestService(t)
	e := testEngineWith(s)
	show := mustCreate(t, s, 1, "A", "2026-10-01T20:00:00Z")

	w := uploadPoster(e, show.ID, pngOf(color.RGBA{200, 10, 10, 255}))
	if w.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	src, path := posterOf(t, w)
	if src != "" || path == "" {
		t.Fatalf("after upload: posterUrl=%q path=%q", src, path)
	}

	// The edit form's full replace, with an empty poster field.
	got, err := s.Update(1, show.ID, domain.LiveShowRequest{Title: "A2", Date: "2026-10-01T20:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Poster == nil || got.Poster.Path != path {
		t.Fatalf("form save dropped the uploaded poster: %+v", got.Poster)
	}

	// Removing it explicitly works.
	req := httptest.NewRequest(http.MethodDelete, "/api/live/shows/"+uintStr(show.ID)+"/poster", nil)
	w = httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("remove: %d %s", w.Code, w.Body)
	}
	if _, p := posterOf(t, w); p != "" {
		t.Errorf("poster still there after remove: %q", p)
	}
}

// A poster from a URL is still dropped when the field is cleared: the rule
// change only concerns posters without a source.
func TestClearingURLPosterStillRemovesIt(t *testing.T) {
	s := newTestService(t)
	srv, _ := posterServer(t)
	show, err := s.Create(1, domain.LiveShowRequest{Title: "A", Date: "2026-10-01T20:00:00Z", PosterURL: srv.URL + "/red.png"})
	if err != nil || show.Poster == nil {
		t.Fatalf("create with poster: %v %+v", err, show.Poster)
	}
	got, err := s.Update(1, show.ID, domain.LiveShowRequest{Title: "A", Date: "2026-10-01T20:00:00Z", PosterURL: ""})
	if err != nil {
		t.Fatal(err)
	}
	if got.Poster != nil {
		t.Errorf("clearing the URL must remove the poster, got %+v", got.Poster)
	}
}

func TestPosterFromPhoto(t *testing.T) {
	s := newTestService(t)
	e := testEngineWith(s)
	show := mustCreate(t, s, 1, "A", "2026-10-01T20:00:00Z")
	photo := mustUpload(t, e, show.ID, pngOf(color.RGBA{10, 10, 200, 255}), nil)

	req := httptest.NewRequest(http.MethodPost, "/api/live/shows/"+uintStr(show.ID)+"/poster/from-photo/"+uintStr(photo.ID), nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("from photo: %d %s", w.Code, w.Body)
	}
	if _, p := posterOf(t, w); p != photo.Image.Path {
		t.Fatalf("poster %q, want the photo's image %q", p, photo.Image.Path)
	}
	// Shared, not copied: deleting the photo leaves the poster in place.
	if err := s.DeletePhoto(1, show.ID, photo.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(1, show.ID)
	if got.Poster == nil || got.Poster.Path != photo.Image.Path {
		t.Errorf("poster lost with the photo: %+v", got.Poster)
	}
}

func TestManualPosterScopingAndRejects(t *testing.T) {
	s := newTestService(t)
	e := testEngineWith(s)
	other := mustCreate(t, s, 2, "B", "2026-10-01T20:00:00Z")
	if w := uploadPoster(e, other.ID, pngOf(color.RGBA{1, 2, 3, 255})); w.Code != http.StatusNotFound {
		t.Errorf("another user's show: %d", w.Code)
	}
	mine := mustCreate(t, s, 1, "A", "2026-10-01T20:00:00Z")
	if w := uploadPoster(e, mine.ID, []byte("not an image")); w.Code != http.StatusBadRequest {
		t.Errorf("bad image: %d %s", w.Code, w.Body)
	}
	if w := uploadPoster(e, mine.ID, nil); w.Code != http.StatusBadRequest {
		t.Errorf("no file: %d", w.Code)
	}
	var n int64
	s.db.Model(&entity.LiveShow{}).Where("id = ? AND poster_id IS NOT NULL", mine.ID).Count(&n)
	if n != 0 {
		t.Error("a rejected upload left a poster")
	}
}
