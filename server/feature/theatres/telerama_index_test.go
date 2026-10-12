package theatres

import (
	"testing"

	"github.com/sbondCo/Watcharr/media/telerama"
)

func TestIndexReviewsDropsConflictingTitles(t *testing.T) {
	idx := indexReviews([]telerama.Review{
		{Title: "Jane", Director: "Brett Morgen", Year: 2017, Rating: 4},
		{Title: "Abyss", Director: "James Cameron", Year: 1989, Rating: 4},
		{Title: "Pressure", Rating: 2},
		{Title: "Abyss", Director: "James Cameron", Year: 2013, Rating: 4},
		{Title: "Jane", Director: "Julian Jarrold", Year: 2007, Rating: 1},
		// A third listing must not bring an ambiguous title back.
		{Title: "JANE", Rating: 4},
	})
	if _, ok := idx["jane"]; ok {
		t.Error("Jane is listed at 4T and 1T: it must get no rating rather than a guess")
	}
	if r, ok := idx["abyss"]; !ok || r.Rating != 4 {
		t.Errorf("Abyss is listed twice at the same rating and should be kept, got %+v (ok=%v)", r, ok)
	}
	if r, ok := idx["pressure"]; !ok || r.Rating != 2 {
		t.Errorf("an unambiguous title should be kept, got %+v (ok=%v)", r, ok)
	}
}
