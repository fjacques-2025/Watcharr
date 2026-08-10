package erakys

import (
	"os"
	"strings"
	"testing"
	"time"
)

// testdata/day_fragment.html is two real film cards, captured verbatim from
// lesvarietes-marseille.com. When the site's template changes, this test is
// what tells you — recapture the fixture and fix the parser against it.
func TestParseDay(t *testing.T) {
	f, err := os.Open("testdata/day_fragment.html")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	films, err := ParseDay(f)
	if err != nil {
		t.Fatalf("ParseDay: %v", err)
	}
	if len(films) != 2 {
		t.Fatalf("got %d films, want 2: %+v", len(films), films)
	}

	got := films[0]
	if got.Title != "GIRL" {
		t.Errorf("title = %q, want %q", got.Title, "GIRL")
	}
	if got.Code == "" {
		t.Error("film code is empty, so showtimes cannot be attributed")
	}
	if len(got.Showtimes) != 1 {
		t.Fatalf("got %d showtimes, want 1: %+v", len(got.Showtimes), got.Showtimes)
	}

	s := got.Showtimes[0]
	if s.Start != "20:15" {
		t.Errorf("start = %q, want %q", s.Start, "20:15")
	}
	if s.End != "22:19" {
		t.Errorf("end = %q, want %q", s.End, "22:19")
	}
	if s.Version != "VOST" {
		t.Errorf("version = %q, want %q", s.Version, "VOST")
	}
	if !strings.Contains(s.BookingURL, "ticketingcine.com") {
		t.Errorf("booking url = %q, want a ticketingcine.com link", s.BookingURL)
	}

	// Genre and runtime come from one "Drame | 01h55" node and must be split.
	if films[1].Title != "DES FLEURS POUR TOKYO" {
		t.Errorf("second title = %q", films[1].Title)
	}
	if films[1].Runtime != 115 {
		t.Errorf("runtime = %d, want 115", films[1].Runtime)
	}
	if films[1].Genre != "Drame" {
		t.Errorf("genre = %q, want %q", films[1].Genre, "Drame")
	}
}

func TestMondayOf(t *testing.T) {
	// The endpoint needs the Monday of the week the day falls in.
	for _, tc := range []struct{ day, want string }{
		{"2026-08-10", "2026-08-10"}, // a Monday
		{"2026-08-16", "2026-08-10"}, // the Sunday after it
		{"2026-08-11", "2026-08-10"},
	} {
		d, err := time.Parse(time.DateOnly, tc.day)
		if err != nil {
			t.Fatal(err)
		}
		if got := mondayOf(d).Format(time.DateOnly); got != tc.want {
			t.Errorf("mondayOf(%s) = %s, want %s", tc.day, got, tc.want)
		}
	}
}

func TestSplitGenreRuntime(t *testing.T) {
	for _, tc := range []struct {
		in      string
		genre   string
		runtime int
	}{
		{"Drame | 01h55", "Drame", 115},
		{"Comédie | 1h30", "Comédie", 90},
		{"Documentaire", "Documentaire", 0},
		{"", "", 0},
	} {
		g, r := splitGenreRuntime(tc.in)
		if g != tc.genre || r != tc.runtime {
			t.Errorf("splitGenreRuntime(%q) = (%q, %d), want (%q, %d)",
				tc.in, g, r, tc.genre, tc.runtime)
		}
	}
}
