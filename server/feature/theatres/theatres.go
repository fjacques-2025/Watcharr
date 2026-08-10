// Package theatres lists what is screening at the user's own cinemas.
//
// Scope, deliberately: the venues are hard coded below. Making them
// user-editable needs a table, a settings screen and a venue search; none of
// that is worth building before the data path has proven itself.
package theatres

import (
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode"

	gocache "github.com/robfig/go-cache"
	"github.com/sbondCo/Watcharr/cache"
	"github.com/sbondCo/Watcharr/domain"
	"github.com/sbondCo/Watcharr/media/erakys"
	"github.com/sbondCo/Watcharr/media/tmdb"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// The user's cinemas. Both run on Erakys, so one integration covers them.
var venues = []erakys.Venue{
	{Name: "Les Variétés", Host: "lesvarietes-marseille.com"},
	{Name: "Artplexe Canebière", Host: "artplexe-canebiere.com"},
}

// Assembled days, keyed by day + region. Short enough that an evening
// programme change shows up, long enough to stay off the cinemas' backs.
const cacheTTL = 3 * time.Hour

var dayStore = gocache.New(cacheTTL, 10*time.Minute)

type Service struct {
	tmdb *tmdb.TMDB
}

func NewService(tmdb *tmdb.TMDB) *Service {
	return &Service{tmdb: tmdb}
}

// Showtimes lists the day's screenings across the configured venues, each film
// matched to TMDB where possible.
//
// A venue that fails is logged and skipped rather than failing the request: one
// cinema's site being down shouldn't blank out the other's programme.
func (s *Service) Showtimes(
	r domain.TheatresRequest,
	region string,
) (domain.TheatresResponse, error) {
	day := time.Now()
	if r.Day != "" {
		d, err := time.Parse(time.DateOnly, r.Day)
		if err != nil {
			return domain.TheatresResponse{}, errors.New("day must be formatted YYYY-MM-DD")
		}
		day = d
	}
	dayStr := day.Format(time.DateOnly)

	resp := domain.TheatresResponse{Day: dayStr}
	for _, v := range venues {
		resp.Theatres = append(resp.Theatres, v.Name)
	}

	cacheKey := cache.CreateCacheKey("TheatreShowtimes", dayStr, region)
	cached := new(domain.TheatresResponse)
	if cache.GetCache(dayStore, cacheKey, &cached) {
		slog.Debug("theatres: Showtimes returning cache.", "day", dayStr)
		return *cached, nil
	}

	// Group by normalized title: the same film plays at both venues, and the
	// Erakys film code is per-site so it can't join across them.
	type entry struct {
		film       *domain.TheatreFilm
		screenings map[string]*domain.TheatreScreenings
	}
	var order []string
	byTitle := make(map[string]*entry)

	for _, v := range venues {
		films, err := erakys.FetchDay(v.Host, day)
		if err != nil {
			slog.Error("theatres: skipping venue", "venue", v.Name, "error", err)
			continue
		}
		for _, f := range films {
			k := normalizeTitle(f.Title)
			e, ok := byTitle[k]
			if !ok {
				e = &entry{
					film: &domain.TheatreFilm{
						Title:   f.Title,
						Genre:   f.Genre,
						Runtime: f.Runtime,
					},
					screenings: map[string]*domain.TheatreScreenings{},
				}
				byTitle[k] = e
				order = append(order, k)
			}
			// Prefer whichever venue published the metadata.
			if e.film.Genre == "" {
				e.film.Genre = f.Genre
			}
			if e.film.Runtime == 0 {
				e.film.Runtime = f.Runtime
			}
			sc, ok := e.screenings[v.Name]
			if !ok {
				sc = &domain.TheatreScreenings{Theatre: v.Name}
				e.screenings[v.Name] = sc
			}
			sc.Showtimes = append(sc.Showtimes, f.Showtimes...)
		}
	}

	if len(order) == 0 {
		slog.Info("theatres: no screenings found", "day", dayStr)
		return resp, nil
	}

	// One TMDB lookup per distinct film, against the region's current
	// theatrical releases first — a much smaller and better targeted set than
	// full-text search, so far less room to pick the wrong film.
	nowPlaying := s.nowPlayingIndex(region)
	for _, k := range order {
		e := byTitle[k]
		e.film.Media = s.matchFilm(k, e.film.Title, nowPlaying)
		for _, v := range venues {
			if sc, ok := e.screenings[v.Name]; ok {
				sort.Slice(sc.Showtimes, func(i, j int) bool {
					return sc.Showtimes[i].Start < sc.Showtimes[j].Start
				})
				e.film.Screenings = append(e.film.Screenings, *sc)
			}
		}
		resp.Films = append(resp.Films, *e.film)
	}

	// Busiest first: at an art-house cinema the one-off screening is the
	// exception, and burying the main programme under it reads as noise.
	sort.SliceStable(resp.Films, func(i, j int) bool {
		return countShowtimes(resp.Films[i]) > countShowtimes(resp.Films[j])
	})

	dayStore.Set(cacheKey, &resp, cacheTTL)
	return resp, nil
}

// nowPlayingIndex maps normalized title -> TMDB media for what is currently in
// theatres in `region`. Returns nil (not an error) on failure: matching then
// falls back to search, which is worse but still works.
func (s *Service) nowPlayingIndex(region string) map[string]domain.Media {
	idx := map[string]domain.Media{}
	// Two pages is ~40 titles, comfortably more than two cinemas screen.
	for page := 1; page <= 2; page++ {
		res, err := s.tmdb.DiscoverMovies(
			tmdb.DiscoverOptions{
				ReleaseDateMin:  time.Now().AddDate(0, 0, -60),
				ReleaseDateMax:  time.Now().AddDate(0, 0, 2),
				WithReleaseType: "2|3",
			},
			page,
			region,
		)
		if err != nil {
			slog.Error("theatres: now-playing index failed, will fall back to search",
				"page", page, "error", err)
			return idx
		}
		for _, v := range res.Results {
			m := v.AsMedia()
			// Index the original title too: cinemas list some films under it,
			// especially in the original-language programming of art houses.
			for _, k := range []string{normalizeTitle(m.Name), normalizeTitle(v.OriginalTitle)} {
				if k == "" {
					continue
				}
				if _, exists := idx[k]; !exists {
					idx[k] = m
				}
			}
		}
		if page >= res.TotalPages {
			break
		}
	}
	return idx
}

// matchFilm identifies a cinema listing on TMDB, or returns nil.
//
// Returning nil is a real outcome, not a failure: retrospectives and one-off
// screenings often aren't in the current-releases window, and a wrong poster
// is worse than none.
func (s *Service) matchFilm(
	key, title string,
	nowPlaying map[string]domain.Media,
) *domain.Media {
	if m, ok := nowPlaying[key]; ok {
		return &m
	}
	res, err := s.tmdb.SearchMovies(tmdb.SearchMoviesOptions{
		SearchUniversalOptions: tmdb.SearchUniversalOptions{Query: title, Page: 1},
	})
	if err != nil {
		slog.Error("theatres: search fallback failed", "title", title, "error", err)
		return nil
	}
	// Only an exact title match counts here — on the localised title or the
	// original one. Search happily returns something for anything, and "close
	// enough" would attach the wrong film silently.
	for i := range res.Results {
		m := res.Results[i].AsMedia()
		if normalizeTitle(m.Name) == key ||
			normalizeTitle(res.Results[i].OriginalTitle) == key {
			return &m
		}
	}
	slog.Info("theatres: no TMDB match", "title", title)
	return nil
}

func countShowtimes(f domain.TheatreFilm) int {
	n := 0
	for _, sc := range f.Screenings {
		n += len(sc.Showtimes)
	}
	return n
}

var titleCleaner = transform.Chain(
	norm.NFD,
	runes.Remove(runes.In(unicode.Mn)), // strip accents
	norm.NFC,
)

// normalizeTitle makes cinema titles ("LA PAT' PATROUILLE : LE FILM") and TMDB
// titles ("La Pat’ Patrouille : le film") comparable: accents, case,
// punctuation and spacing all differ between the two.
func normalizeTitle(s string) string {
	s, _, err := transform.String(titleCleaner, s)
	if err != nil {
		slog.Debug("theatres: title normalization failed, using raw", "error", err)
	}
	var b strings.Builder
	lastSpace := true
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastSpace = false
		case !lastSpace:
			b.WriteRune(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}
