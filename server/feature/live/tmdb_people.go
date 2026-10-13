package live

import (
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/sbondCo/Watcharr/domain"
	"github.com/sbondCo/Watcharr/media/tmdb"
)

// errTmdbUnavailable: the search itself failed, there is nothing to show.
var errTmdbUnavailable = errors.New("TMDB search failed")

const (
	minTmdbQuery       = 2
	maxTmdbCandidates  = 8
	maxKnownFor        = 3
	maxCreditsInFlight = 4
)

// tmdbPeople is the slice of the TMDB client the notebook uses, an interface
// so tests run without the network.
type tmdbPeople interface {
	LiveSearchPeople(q string) ([]tmdb.LivePersonSearchResult, error)
	LivePersonCreditRefs(id int) (tmdb.LiveCreditRefs, error)
}

type mediaKey struct {
	mediaType string
	id        int
}

// TmdbPeople lists TMDB people the user may link an artist to, with what
// tells homonyms apart: known works, and how many of theirs the user has in
// their list.
func (s *Service) TmdbPeople(userId uint, q string) ([]domain.LiveTmdbPerson, error) {
	out := []domain.LiveTmdbPerson{}
	q = strings.TrimSpace(q)
	if utf8.RuneCountInString(q) < minTmdbQuery {
		return out, nil
	}
	found, err := s.tmdb.LiveSearchPeople(q)
	if err != nil {
		slog.Error("live: TmdbPeople search failed", "error", err)
		return nil, errTmdbUnavailable
	}
	if len(found) > maxTmdbCandidates {
		found = found[:maxTmdbCandidates]
	}
	if len(found) == 0 {
		return out, nil
	}
	mine, err := s.listedMedia(userId)
	if err != nil {
		return nil, err
	}
	counts := s.countInList(found, mine)
	for i, p := range found {
		out = append(out, domain.LiveTmdbPerson{
			TmdbID:      p.ID,
			Name:        p.Name,
			ProfilePath: p.ProfilePath,
			Department:  p.KnownForDepartment,
			KnownFor:    knownFor(p.KnownFor),
			InYourList:  counts[i],
		})
	}
	// Stable: TMDB's relevance order stays the tie-break.
	sort.SliceStable(out, func(i, j int) bool { return out[i].InYourList > out[j].InYourList })
	return out, nil
}

// listedMedia is the set of movies and series in the user's watched list.
func (s *Service) listedMedia(userId uint) (map[mediaKey]bool, error) {
	var rows []struct {
		TmdbID int
		Type   string
	}
	err := s.db.Table("watcheds AS w").
		Select("c.tmdb_id, c.type").
		Joins("JOIN contents AS c ON c.id = w.content_id").
		Where("w.user_id = ? AND w.deleted_at IS NULL AND c.type IN ?", userId, []string{"movie", "tv"}).
		Scan(&rows).Error
	if err != nil {
		slog.Error("live: listedMedia failed", "user_id", userId, "error", err)
		return nil, err
	}
	set := make(map[mediaKey]bool, len(rows))
	for _, r := range rows {
		set[mediaKey{r.Type, r.TmdbID}] = true
	}
	return set, nil
}

// countInList fetches every candidate's credits, a few at a time so a search
// does not fire 8 TMDB calls at once. A failed fetch counts 0: one person's
// missing hint must not hide the others.
func (s *Service) countInList(found []tmdb.LivePersonSearchResult, mine map[mediaKey]bool) []int {
	counts := make([]int, len(found))
	sem := make(chan struct{}, maxCreditsInFlight)
	var wg sync.WaitGroup
	for i, p := range found {
		wg.Add(1)
		sem <- struct{}{}
		go func(i, id int) {
			defer wg.Done()
			defer func() { <-sem }()
			refs, err := s.tmdb.LivePersonCreditRefs(id)
			if err != nil {
				slog.Warn("live: TMDB person credits failed", "tmdb_id", id, "error", err)
				return
			}
			// Someone both acting in and directing a film counts it once.
			seen := map[mediaKey]bool{}
			for _, list := range [][]tmdb.LiveCreditRef{refs.Cast, refs.Crew} {
				for _, r := range list {
					k := mediaKey{r.MediaType, r.ID}
					if mine[k] {
						seen[k] = true
					}
				}
			}
			counts[i] = len(seen)
		}(i, p.ID)
	}
	wg.Wait()
	return counts
}

func knownFor(in []tmdb.LiveKnownFor) []domain.LiveTmdbKnownFor {
	out := []domain.LiveTmdbKnownFor{}
	for _, k := range in {
		if len(out) == maxKnownFor {
			break
		}
		var title, date string
		switch k.MediaType {
		case "movie":
			title, date = firstNonEmpty(k.Title, k.Name), k.ReleaseDate
		case "tv":
			title, date = firstNonEmpty(k.Name, k.Title), k.FirstAirDate
		default:
			// The contract only knows movies and series.
			continue
		}
		out = append(out, domain.LiveTmdbKnownFor{Title: title, Year: yearOf(date), MediaType: k.MediaType})
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// yearOf reads the year of a TMDB "YYYY-MM-DD" date; nil when absent.
func yearOf(date string) *int {
	if len(date) < 4 {
		return nil
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil || y <= 0 {
		return nil
	}
	return &y
}
