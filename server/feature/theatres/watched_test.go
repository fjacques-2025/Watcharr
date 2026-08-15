package theatres

import (
	"errors"
	"testing"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"github.com/sbondCo/Watcharr/feature/watched/addedtocontent"
	"github.com/sbondCo/Watcharr/util"
)

type fakeWatched struct {
	items []entity.Watched
	err   error
}

func (f fakeWatched) GetWatchedItemBySupportedMediaId(
	_ uint, _ uint, _ util.SupportedMedia,
) (entity.Watched, error) {
	return entity.Watched{}, errors.New("not used")
}

func (f fakeWatched) GetWatchedItemsBySupportedMediaIds(
	_ uint, _ []addedtocontent.IdToTypePair,
) ([]entity.Watched, error) {
	return f.items, f.err
}

func movieEntry(tmdbID int, status entity.WatchedStatus) entity.Watched {
	return entity.Watched{
		Status:  status,
		Content: &entity.Content{TmdbID: tmdbID, Type: entity.MOVIE},
	}
}

func programmeFixture() domain.TheatresResponse {
	return domain.TheatresResponse{
		Day:      "2026-08-11",
		Theatres: []string{"Les Variétés"},
		Films: []domain.TheatreFilm{
			{Title: "SEEN ONE", Media: &domain.Media{
				Type: domain.MediaTypeTMDBMovie,
				IDs:  domain.MediaIDs{TMDB: 111},
			}},
			{Title: "NOT ON MY LIST", Media: &domain.Media{
				Type: domain.MediaTypeTMDBMovie,
				IDs:  domain.MediaIDs{TMDB: 222},
			}},
			// Unidentified on TMDB (a retrospective): must survive untouched.
			{Title: "COWBOY BEBOP"},
		},
	}
}

func TestWithWatchedAttachesToTheRightFilm(t *testing.T) {
	s := NewService(nil, fakeWatched{
		items: []entity.Watched{movieEntry(111, entity.FINISHED)},
	})

	got := s.withWatched(1, programmeFixture())

	if got.Films[0].Watched == nil {
		t.Fatal("the film on the list got no watched data")
	}
	if got.Films[0].Watched.Status != entity.FINISHED {
		t.Errorf("status = %q, want FINISHED", got.Films[0].Watched.Status)
	}
	if got.Films[1].Watched != nil {
		t.Error("a film that isn't on the list was marked as watched")
	}
	if got.Films[2].Watched != nil {
		t.Error("a film with no TMDB match was marked as watched")
	}
}

// The programme is cached and shared between users, so attaching one user's
// list data must not write into it. This is the test that matters: getting it
// wrong leaks what you've watched to whoever asks next.
func TestWithWatchedDoesNotMutateTheSharedProgramme(t *testing.T) {
	s := NewService(nil, fakeWatched{
		items: []entity.Watched{movieEntry(111, entity.FINISHED)},
	})

	shared := programmeFixture()
	got := s.withWatched(1, shared)

	if got.Films[0].Watched == nil {
		t.Fatal("precondition failed: nothing was attached")
	}
	for i, f := range shared.Films {
		if f.Watched != nil {
			t.Errorf("shared programme film %d was mutated (%q)", i, f.Title)
		}
	}
	if &got.Films[0] == &shared.Films[0] {
		t.Error("the films slice was not copied, so a later write would leak")
	}
}

func TestWithWatchedSurvivesAProviderError(t *testing.T) {
	s := NewService(nil, fakeWatched{err: errors.New("db down")})

	got := s.withWatched(1, programmeFixture())

	// Showtimes without the marks are still showtimes.
	if len(got.Films) != 3 {
		t.Fatalf("got %d films, want 3", len(got.Films))
	}
	for i, f := range got.Films {
		if f.Watched != nil {
			t.Errorf("film %d marked despite the lookup failing", i)
		}
	}
}

func TestWithWatchedWithoutProvider(t *testing.T) {
	s := NewService(nil, nil)
	if got := s.withWatched(1, programmeFixture()); len(got.Films) != 3 {
		t.Fatalf("got %d films, want 3", len(got.Films))
	}
}
