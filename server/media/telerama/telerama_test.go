package telerama

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// testdata/list_fragment.html is three real cards captured verbatim from
// telerama.fr/liste/films/2t/. When their template changes, this test is what
// tells you — recapture the fixture and fix the parser against it.
func TestParseList(t *testing.T) {
	f, err := os.Open("testdata/list_fragment.html")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	reviews, err := ParseList(f)
	if err != nil {
		t.Fatalf("ParseList: %v", err)
	}
	if len(reviews) != 3 {
		t.Fatalf("got %d reviews, want 3: %+v", len(reviews), reviews)
	}

	for i, r := range reviews {
		if r.Title == "" {
			t.Errorf("review %d has no title", i)
		}
		if !strings.HasPrefix(r.URL, "https://www.telerama.fr/cinema/") {
			t.Errorf("review %d url = %q, want an absolute telerama cinema url", i, r.URL)
		}
		if !strings.Contains(r.URL, "_cri-") {
			t.Errorf("review %d url = %q, want a review (_cri-) url", i, r.URL)
		}
		// Rating is set by the caller from the bucket, so it must stay 0 here.
		if r.Rating != 0 || r.RatingLabel != "" {
			t.Errorf("review %d carries a rating (%d/%q); ParseList must not set one",
				i, r.Rating, r.RatingLabel)
		}
	}

	// The director sits in the first author span, the year in the second.
	var girl *Review
	for i := range reviews {
		if reviews[i].Title == "Girl" {
			girl = &reviews[i]
		}
	}
	if girl == nil {
		t.Fatalf("no review titled \"Girl\" in %+v", reviews)
	}
	if girl.Director == "" {
		t.Error("Girl has no director")
	}
	if girl.Year == 0 {
		t.Error("Girl has no year")
	}
}

func TestPageURL(t *testing.T) {
	for _, c := range []struct {
		page int
		want string
	}{
		{1, "https://www.telerama.fr/liste/films/3t/"},
		{2, "https://www.telerama.fr/liste/films/3t/2/"},
		{5, "https://www.telerama.fr/liste/films/3t/5/"},
	} {
		if got := pageURL("/liste/films/3t/", c.page); got != c.want {
			t.Errorf("page %d: got %q, want %q", c.page, got, c.want)
		}
	}
}

// TestFetchIndexLive hits telerama.fr. Skipped unless asked for:
//
//	WATCHARR_LIVE_TEST=1 go test ./media/telerama/ -run Live -v
func TestFetchIndexLive(t *testing.T) {
	if os.Getenv("WATCHARR_LIVE_TEST") == "" {
		t.Skip("set WATCHARR_LIVE_TEST=1 to run (hits telerama.fr)")
	}
	reviews, err := FetchIndex()
	if err != nil {
		t.Fatalf("FetchIndex: %v", err)
	}
	// Each page holds 50 films. Well under the full count means pages are
	// failing or pagination broke, even if every bucket still answers.
	if want := 5 * pagesPerBucket * 50 * 8 / 10; len(reviews) < want {
		t.Errorf("only %d reviews, expected at least %d from %d pages per bucket "+
			"— pages are failing, pagination moved, or the template changed",
			len(reviews), want, pagesPerBucket)
	}

	// Every rating bucket should contribute, and each film belongs to exactly
	// one: a film in two buckets means the buckets aren't what we think.
	// A title alone doesn't identify a film — "Jane" is both Brett Morgen's
	// 2017 documentary (4T) and Julian Jarrold's 2007 drama (1T), and two
	// different "Kraken" came out in 2026 — so key on director and year too.
	seen := map[int]int{}
	films := map[string]int{}
	for _, r := range reviews {
		seen[r.Rating]++
		k := fmt.Sprintf("%s|%s|%d", r.Title, r.Director, r.Year)
		if prev, dup := films[k]; dup && prev != r.Rating {
			t.Errorf("%q (%s, %d) appears with both %d and %d Ts",
				r.Title, r.Director, r.Year, prev, r.Rating)
		}
		films[k] = r.Rating
	}
	for rating := 0; rating <= 4; rating++ {
		if seen[rating] == 0 {
			t.Errorf("no films found with %d Ts", rating)
		}
	}
	t.Logf("%d reviews: %v", len(reviews), seen)
}
