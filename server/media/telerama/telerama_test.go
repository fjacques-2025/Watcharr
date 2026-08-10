package telerama

import (
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
	if len(reviews) < 50 {
		t.Errorf("only %d reviews across all five buckets, expected far more "+
			"— a bucket probably failed or the template moved", len(reviews))
	}

	// Every rating bucket should contribute, and each film belongs to exactly
	// one: a title in two buckets means the buckets aren't what we think.
	seen := map[int]int{}
	titles := map[string]int{}
	for _, r := range reviews {
		seen[r.Rating]++
		if prev, dup := titles[r.Title]; dup && prev != r.Rating {
			t.Errorf("%q appears with both %d and %d Ts", r.Title, prev, r.Rating)
		}
		titles[r.Title] = r.Rating
	}
	for rating := 0; rating <= 4; rating++ {
		if seen[rating] == 0 {
			t.Errorf("no films found with %d Ts", rating)
		}
	}
	t.Logf("%d reviews: %v", len(reviews), seen)
}
