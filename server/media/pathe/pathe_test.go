package pathe

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakePathe serves the testdata fixtures — verbatim pathe.fr responses for
// 2026-11-09, trimmed to three films: two playing that day, and an opera
// relay that doesn't. When the API changes, recapture them and fix against.
func fakePathe(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	routes := map[string]string{
		"/api/cinema/cinema-pathe-la-joliette/shows":                                        "shows.json",
		"/api/show/godzilla-minus-zero-54837":                                               "show_godzilla.json",
		"/api/show/kraken-50712":                                                            "show_kraken.json",
		"/api/show/godzilla-minus-zero-54837/showtimes/cinema-pathe-la-joliette/2026-11-09": "showtimes_godzilla.json",
		"/api/show/kraken-50712/showtimes/cinema-pathe-la-joliette/2026-11-09":              "showtimes_kraken.json",
	}
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		f, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		b, err := os.ReadFile("testdata/" + f)
		if err != nil {
			t.Errorf("reading fixture: %v", err)
		}
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	old := baseURL
	baseURL = srv.URL
	store.Flush()
	t.Cleanup(func() { baseURL = old; store.Flush() })
	return srv, &calls
}

func TestFetchDay(t *testing.T) {
	_, calls := fakePathe(t)

	day := time.Date(2026, 11, 9, 0, 0, 0, 0, time.Local)
	films, err := FetchDay("cinema-pathe-la-joliette", day)
	if err != nil {
		t.Fatalf("FetchDay: %v", err)
	}
	if len(films) != 2 {
		t.Fatalf("got %d films, want 2 (the opera relay doesn't play that day): %+v", len(films), films)
	}
	sort.Slice(films, func(i, j int) bool { return films[i].Title < films[j].Title })

	g := films[0]
	if g.Title != "Godzilla Minus Zero" || g.Runtime != 135 {
		t.Errorf("title/runtime = %q/%d", g.Title, g.Runtime)
	}
	if g.Genre != "Action, Aventure, Drame" {
		t.Errorf("genre = %q", g.Genre)
	}
	if len(g.Showtimes) != 8 {
		t.Fatalf("got %d showtimes, want 8", len(g.Showtimes))
	}
	s := g.Showtimes[0]
	if s.Start != "13:10" || s.End != "15:45" || s.Version != "VF" {
		t.Errorf("first showtime = %+v, want 13:10-15:45 VF", s)
	}
	if !strings.HasPrefix(s.BookingURL, "https://s.pathe.fr/") {
		t.Errorf("booking url = %q", s.BookingURL)
	}

	if films[1].Title != "Kraken" || len(films[1].Showtimes) != 1 {
		t.Errorf("second film = %+v", films[1])
	}

	// A second day reuses the film list and details: only showtimes are new.
	before := calls.Load()
	if _, err := FetchDay("cinema-pathe-la-joliette", day); err != nil {
		t.Fatalf("second FetchDay: %v", err)
	}
	if n := calls.Load() - before; n != 2 {
		t.Errorf("second fetch made %d requests, want 2 (showtimes only)", n)
	}
}

func TestFetchDaySkipsBrokenFilm(t *testing.T) {
	fakePathe(t)
	// Kraken's details come back without a title: the day still loads.
	baseURLBefore := baseURL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/show/kraken-50712" {
			w.Write([]byte(`{}`))
			return
		}
		http.Redirect(w, r, baseURLBefore+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(srv.Close)
	baseURL = srv.URL

	films, err := FetchDay("cinema-pathe-la-joliette", time.Date(2026, 11, 9, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("FetchDay: %v", err)
	}
	if len(films) != 1 || films[0].Title != "Godzilla Minus Zero" {
		t.Errorf("got %+v, want Godzilla only", films)
	}
}

func TestNormalizeVersion(t *testing.T) {
	for in, want := range map[string]string{"vf": "VF", "vost": "VOST", "vfst": "VF", "vo": "VO", "": ""} {
		if got := normalizeVersion(in); got != want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
