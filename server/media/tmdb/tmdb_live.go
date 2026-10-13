package tmdb

// Fork-only TMDB calls for the live-shows notebook (feature/live). Kept apart
// from the upstream methods: those structs lack fields the notebook needs
// (TV `name`/`first_air_date` in known_for, crew in combined credits).

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/sbondCo/Watcharr/cache"
)

// LiveKnownFor is a known_for item: movies carry title/release_date, TV
// carries name/first_air_date.
type LiveKnownFor struct {
	ID           int    `json:"id"`
	MediaType    string `json:"media_type"`
	Title        string `json:"title"`
	Name         string `json:"name"`
	ReleaseDate  string `json:"release_date"`
	FirstAirDate string `json:"first_air_date"`
}

type LivePersonSearchResult struct {
	ID                 int            `json:"id"`
	Name               string         `json:"name"`
	ProfilePath        string         `json:"profile_path"`
	KnownForDepartment string         `json:"known_for_department"`
	KnownFor           []LiveKnownFor `json:"known_for"`
}

type liveSearchPeopleResponse struct {
	Results []LivePersonSearchResult `json:"results"`
}

// LiveCreditRef is just enough of a combined-credits item to match it
// against the user's list.
type LiveCreditRef struct {
	ID        int    `json:"id"`
	MediaType string `json:"media_type"`
}

type LiveCreditRefs struct {
	Cast []LiveCreditRef `json:"cast"`
	Crew []LiveCreditRef `json:"crew"`
}

// LiveSearchPeople is page 1 of the TMDB people search. Cached an hour per
// (query, language): the upstream SearchPeople key ignores the language.
func (t *TMDB) LiveSearchPeople(q string) ([]LivePersonSearchResult, error) {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil, errors.New("request is invalid")
	}
	lang := t.GetLang()
	cacheKey := cache.CreateCacheKey("LiveSearchPeople", q, lang)
	var out []LivePersonSearchResult
	if cache.GetCache(ContentStore, cacheKey, &out) {
		return out, nil
	}
	resp := new(liveSearchPeopleResponse)
	if err := t.req("/search/person", map[string]string{"query": q, "page": "1"}, &resp); err != nil {
		slog.Error("LiveSearchPeople: Request failed!", "error", err)
		return nil, errors.New("request failed")
	}
	ContentStore.Set(cacheKey, resp.Results, time.Hour)
	return resp.Results, nil
}

// LivePersonCreditRefs is the person's combined credits, cast and crew.
// Language-independent ids, so cached per person like PersonCredits.
func (t *TMDB) LivePersonCreditRefs(id int) (LiveCreditRefs, error) {
	cacheKey := cache.CreateCacheKey("LivePersonCreditRefs", id)
	var out LiveCreditRefs
	if cache.GetCache(ContentStore, cacheKey, &out) {
		return out, nil
	}
	resp := new(LiveCreditRefs)
	if err := t.req("/person/"+strconv.Itoa(id)+"/combined_credits", map[string]string{}, &resp); err != nil {
		slog.Error("LivePersonCreditRefs: Request failed!", "error", err)
		return LiveCreditRefs{}, errors.New("request failed")
	}
	ContentStore.Set(cacheKey, *resp, time.Hour*24)
	return *resp, nil
}
