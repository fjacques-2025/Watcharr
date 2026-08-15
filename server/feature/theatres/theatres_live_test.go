package theatres

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/sbondCo/Watcharr/domain"
	"github.com/sbondCo/Watcharr/media/tmdb"
)

// TestShowtimesLive hits the real cinema sites and TMDB. It is skipped unless
// you ask for it, since it depends on the network and on today's programme:
//
//	WATCHARR_LIVE_TEST=1 TMDB_KEY=… go test ./feature/theatres/ -run Live -v
//
// Run it when the Erakys template may have changed, or when films stop being
// matched to TMDB. It asserts only what must hold on any day — showtimes exist
// and are well formed — and prints the match rate for you to eyeball.
func TestShowtimesLive(t *testing.T) {
	if os.Getenv("WATCHARR_LIVE_TEST") == "" {
		t.Skip("set WATCHARR_LIVE_TEST=1 to run (hits real cinema sites and TMDB)")
	}
	key := os.Getenv("TMDB_KEY")
	if key == "" {
		t.Skip("set TMDB_KEY to run")
	}
	lang := os.Getenv("TMDB_LANG")
	if lang == "" {
		lang = "fr-FR"
	}

	// nil watched provider: this test exercises the scraping and matching
	// path, not the per-user list data, and withWatched no-ops without one.
	s := NewService(tmdb.NewTMDB(key, lang), nil)
	resp, err := s.Showtimes(domain.TheatresRequest{}, "FR", 0)
	if err != nil {
		t.Fatalf("Showtimes: %v", err)
	}
	if len(resp.Films) == 0 {
		t.Fatal("no films at all — the parser or the endpoint has broken " +
			"(a day with zero screenings across both venues is not plausible)")
	}

	matched := 0
	for _, f := range resp.Films {
		if f.Title == "" {
			t.Error("film with an empty title")
		}
		if len(f.Screenings) == 0 {
			t.Errorf("%q has no screenings but was kept", f.Title)
		}
		for _, sc := range f.Screenings {
			if len(sc.Showtimes) == 0 {
				t.Errorf("%q at %q has an empty screenings entry", f.Title, sc.Theatre)
			}
			for _, st := range sc.Showtimes {
				if _, err := time.Parse("15:04", st.Start); err != nil {
					t.Errorf("%q at %q: bad start time %q", f.Title, sc.Theatre, st.Start)
				}
			}
		}
		if f.Media != nil {
			matched++
		}
	}

	b, _ := json.MarshalIndent(resp, "", "  ")
	t.Logf("day=%s films=%d matched to TMDB=%d/%d\n%s",
		resp.Day, len(resp.Films), matched, len(resp.Films), b)
}
