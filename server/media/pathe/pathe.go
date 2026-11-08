// Package pathe reads cinema programming from pathe.fr.
//
// There is no public API, but the site's pages are rendered from a JSON API
// under /api that answers without a key. Three calls make a day:
//
//	/api/cinema/<cinema>/shows                      every film and the days it plays
//	/api/show/<film>                                title, running time, genres
//	/api/show/<film>/showtimes/<cinema>/<YYYY-MM-DD> that day's screenings
//
// That is one call per film per day, so film details are cached for a long
// time and the film list for an hour. Callers must still cache the assembled
// day. The fixtures in testdata are verbatim responses; TestFetchDay is what
// tells you the API changed.
//
// Results use the erakys types, which are what the theatres feature consumes.
package pathe

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	cache "github.com/robfig/go-cache"
	"github.com/sbondCo/Watcharr/media/erakys"
)

const (
	// Identify ourselves rather than pretending to be a browser.
	userAgent = "Watcharr (self-hosted watch list; showtimes for the user's own cinemas)"
	// Film details only change when Pathé edits the page, which is rare.
	detailsTTL = 24 * time.Hour
	// The film list gains new entries during the week.
	listTTL = time.Hour
	// Concurrent requests for one day. Enough to keep a cold load to a few
	// seconds, few enough not to hammer the site.
	workers = 4
)

// Var so tests can point it at an httptest server.
var baseURL = "https://www.pathe.fr"

var httpClient = &http.Client{Timeout: 20 * time.Second}

var store = cache.New(detailsTTL, time.Hour)

type showsResponse struct {
	Shows map[string]struct {
		Days map[string]json.RawMessage `json:"days"`
	} `json:"shows"`
}

type showDetails struct {
	Title    string   `json:"title"`
	Duration int      `json:"duration"`
	Genres   []string `json:"genres"`
}

type showtime struct {
	// "2026-11-09 13:10:00", local to the cinema.
	Time    string `json:"time"`
	EndTime string `json:"endTime"`
	// "vf", "vost", "vfst", "vo".
	Version string `json:"version"`
	RefCmd  string `json:"refCmd"`
}

// FetchDay returns everything screening at `cinema` (a pathe.fr slug such as
// "cinema-pathe-la-joliette") on `day`.
//
// A film whose details or showtimes fail to load is skipped rather than
// failing the day: one bad entry shouldn't hide the rest of the programme.
func FetchDay(cinema string, day time.Time) ([]erakys.Film, error) {
	dayStr := day.Format(time.DateOnly)
	slugs, err := filmsOn(cinema, dayStr)
	if err != nil {
		return nil, err
	}

	films := make([]*erakys.Film, len(slugs))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				f, err := fetchFilm(cinema, slugs[i], dayStr)
				if err != nil {
					slog.Error("pathe: skipping film", "film", slugs[i], "error", err)
					continue
				}
				films[i] = f
			}
		}()
	}
	for i := range slugs {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	var out []erakys.Film
	for _, f := range films {
		if f != nil && len(f.Showtimes) > 0 {
			out = append(out, *f)
		}
	}
	slog.Debug("pathe: FetchDay done", "cinema", cinema, "day", dayStr, "films", len(out))
	return out, nil
}

// filmsOn returns the slugs of the films playing at `cinema` on `day`.
func filmsOn(cinema, day string) ([]string, error) {
	key := "shows:" + cinema
	var shows showsResponse
	if v, ok := store.Get(key); ok {
		shows = v.(showsResponse)
	} else {
		if err := getJSON("/api/cinema/"+url.PathEscape(cinema)+"/shows", &shows); err != nil {
			return nil, err
		}
		store.Set(key, shows, listTTL)
	}
	var slugs []string
	for slug, s := range shows.Shows {
		if _, ok := s.Days[day]; ok {
			slugs = append(slugs, slug)
		}
	}
	return slugs, nil
}

func fetchFilm(cinema, slug, day string) (*erakys.Film, error) {
	d, err := details(slug)
	if err != nil {
		return nil, err
	}
	var sts []showtime
	path := fmt.Sprintf("/api/show/%s/showtimes/%s/%s",
		url.PathEscape(slug), url.PathEscape(cinema), day)
	if err := getJSON(path, &sts); err != nil {
		return nil, err
	}
	f := &erakys.Film{
		Code:    slug,
		Title:   d.Title,
		Genre:   strings.Join(d.Genres, ", "),
		Runtime: d.Duration,
	}
	for _, st := range sts {
		start := clock(st.Time)
		if start == "" {
			continue
		}
		f.Showtimes = append(f.Showtimes, erakys.Showtime{
			Start:      start,
			End:        clock(st.EndTime),
			Version:    normalizeVersion(st.Version),
			BookingURL: st.RefCmd,
		})
	}
	return f, nil
}

func details(slug string) (showDetails, error) {
	key := "show:" + slug
	if v, ok := store.Get(key); ok {
		return v.(showDetails), nil
	}
	var d showDetails
	if err := getJSON("/api/show/"+url.PathEscape(slug), &d); err != nil {
		return d, err
	}
	if d.Title == "" {
		return d, errors.New("film details without a title")
	}
	store.Set(key, d, detailsTTL)
	return d, nil
}

func getJSON(path string, v any) error {
	req, err := http.NewRequest(http.MethodGet, baseURL+path+"?language=fr", nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Error("pathe: request failed", "path", path, "error", err)
		return errors.New("pathe request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pathe request %s returned %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("decoding %s: %w", path, err)
	}
	return nil
}

// "2026-11-09 13:10:00" -> "13:10". Empty when it doesn't parse.
func clock(s string) string {
	t, err := time.Parse(time.DateTime, s)
	if err != nil {
		return ""
	}
	return t.Format("15:04")
}

// Same vocabulary as erakys. "vfst" (French with French subtitles, for the
// hard of hearing) is still the French version.
func normalizeVersion(s string) string {
	switch v := strings.ToUpper(strings.TrimSpace(s)); {
	case strings.HasPrefix(v, "VOST"):
		return "VOST"
	case strings.HasPrefix(v, "VO"):
		return "VO"
	case strings.HasPrefix(v, "VF"):
		return "VF"
	default:
		return ""
	}
}
