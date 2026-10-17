package live

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/sbondCo/Watcharr/domain"
)

func TestStripDomainSuffix(t *testing.T) {
	for _, c := range []struct{ title, host, want string }{
		{"TRANScenDANCE & L'Un contre l'Autre • KLAP Maison pour la danse - Kelemenis & cie", "www.kelemenis.fr", "TRANScenDANCE & L'Un contre l'Autre"},
		// The real title's own " - " is kept: " • " is the stronger separator.
		{"Roméo - Juliette • Théâtre La Criée", "theatre-lacriee.com", "Roméo - Juliette"},
		{"Bâtir - Théâtre Joliette", "www.theatrejoliette.fr", "Bâtir"},
		// The domain's generic words don't count ("theatre", "opera").
		{"Le Théâtre de la ville - Théâtre", "theatre-ville.fr", "Le Théâtre de la ville - Théâtre"},
		// Nothing naming the site: untouched.
		{"Boat People", "theatre-lacriee.com", "Boat People"},
		{"Hamlet - Shakespeare", "www.example.org", "Hamlet - Shakespeare"},
	} {
		if got := stripDomainSuffix(c.title, c.host); got != c.want {
			t.Errorf("stripDomainSuffix(%q, %q) = %q, want %q", c.title, c.host, got, c.want)
		}
	}
}

func TestTransientFetchError(t *testing.T) {
	for err, want := range map[error]bool{
		errors.New("HTTP 503"):                   true,
		errors.New("context deadline exceeded"):  true,
		errors.New("connection reset by peer"):   true,
		errors.New("HTTP 404"):                   false,
		errBlockedAddress:                        false,
		errTooBig:                                false,
		errors.New("too many redirects"):         false,
		&ValidationError{Msg: "not a valid url"}: false,
	} {
		if got := transientFetchError(err); got != want {
			t.Errorf("transientFetchError(%v) = %v, want %v", err, got, want)
		}
	}
}

// A poster download failing once with a server error is retried; a 404 isn't.
func TestPosterDownloadRetry(t *testing.T) {
	prev := posterRetryDelay
	posterRetryDelay = 0
	t.Cleanup(func() { posterRetryDelay = prev })

	img := pngOf(red)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		switch r.URL.Path {
		case "/flaky.png":
			if n == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Write(img)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	s := newTestService(t)
	show, err := s.Create(1, domain.LiveShowRequest{Title: "A", Date: "2026-11-05T20:00:00Z", PosterURL: srv.URL + "/flaky.png"})
	if err != nil || show.Poster == nil {
		t.Fatalf("flaky poster not saved after a retry: %v %+v", err, show.Poster)
	}
	if hits.Load() != 2 {
		t.Errorf("hits = %d, want 2 (one retry)", hits.Load())
	}

	hits.Store(0)
	if _, err := s.Create(1, domain.LiveShowRequest{Title: "B", Date: "2026-11-05T20:00:00Z", PosterURL: srv.URL + "/missing.png"}); err == nil {
		t.Fatal("a 404 poster must be refused")
	}
	if hits.Load() != 1 {
		t.Errorf("a 404 was retried: %d hits", hits.Load())
	}
}
