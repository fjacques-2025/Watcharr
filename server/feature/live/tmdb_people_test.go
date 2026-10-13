package live

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/media/tmdb"
)

// fakeTmdb stands in for the TMDB client: no network in tests.
type fakeTmdb struct {
	people    []tmdb.LivePersonSearchResult
	searchErr error
	credits   map[int]tmdb.LiveCreditRefs
	failFor   map[int]bool

	mu                    sync.Mutex
	queries               []string
	inFlight, maxInFlight int
}

func (f *fakeTmdb) LiveSearchPeople(q string) ([]tmdb.LivePersonSearchResult, error) {
	f.mu.Lock()
	f.queries = append(f.queries, q)
	f.mu.Unlock()
	return f.people, f.searchErr
}

func (f *fakeTmdb) LivePersonCreditRefs(id int) (tmdb.LiveCreditRefs, error) {
	f.mu.Lock()
	f.inFlight++
	if f.inFlight > f.maxInFlight {
		f.maxInFlight = f.inFlight
	}
	f.mu.Unlock()
	// Long enough for the other goroutines to pile up if nothing limits them.
	time.Sleep(5 * time.Millisecond)
	f.mu.Lock()
	f.inFlight--
	f.mu.Unlock()
	if f.failFor[id] {
		return tmdb.LiveCreditRefs{}, errors.New("boom")
	}
	return f.credits[id], nil
}

func ref(mediaType string, id int) tmdb.LiveCreditRef {
	return tmdb.LiveCreditRef{ID: id, MediaType: mediaType}
}

// addWatched puts a movie or series in a user's list; deleted soft-deletes it.
func addWatched(t *testing.T, s *Service, userId uint, ct entity.ContentType, tmdbId int, deleted bool) {
	t.Helper()
	var c entity.Content
	if err := s.db.Where(entity.Content{TmdbID: tmdbId, Type: ct}).FirstOrCreate(&c).Error; err != nil {
		t.Fatal(err)
	}
	w := entity.Watched{UserID: userId, ContentID: &c.ID, Status: entity.FINISHED}
	if err := s.db.Create(&w).Error; err != nil {
		t.Fatal(err)
	}
	if deleted {
		if err := s.db.Delete(&w).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func withFake(t *testing.T, f *fakeTmdb) *Service {
	t.Helper()
	s := newTestService(t)
	s.tmdb = f
	return s
}

func TestTmdbPeopleShortQuery(t *testing.T) {
	f := &fakeTmdb{people: []tmdb.LivePersonSearchResult{{ID: 1, Name: "A"}}}
	s := withFake(t, f)
	for _, q := range []string{"", " ", "a", "  é  "} {
		got, err := s.TmdbPeople(1, q)
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("q=%q: %#v %v, want [] nil", q, got, err)
		}
	}
	if len(f.queries) != 0 {
		t.Errorf("TMDB searched for %v", f.queries)
	}
	if _, err := s.TmdbPeople(1, "  ab  "); err != nil || len(f.queries) != 1 || f.queries[0] != "ab" {
		t.Errorf("2 chars: queries=%v err=%v", f.queries, err)
	}
}

func TestTmdbPeopleMapping(t *testing.T) {
	f := &fakeTmdb{people: []tmdb.LivePersonSearchResult{
		{ID: 10, Name: "Isabelle Huppert", ProfilePath: "/ih.jpg", KnownForDepartment: "Acting", KnownFor: []tmdb.LiveKnownFor{
			{ID: 1, MediaType: "movie", Title: "La Pianiste", ReleaseDate: "2001-09-05"},
			{ID: 2, MediaType: "tv", Name: "Dix pour cent", FirstAirDate: "2015-10-14"},
			{ID: 3, MediaType: "movie", Title: "Sans date", ReleaseDate: ""},
			{ID: 4, MediaType: "movie", Title: "Quatrième", ReleaseDate: "1990-01-01"},
		}},
		{ID: 11, Name: "Nobody"},
		{ID: 12, Name: "Odd", KnownFor: []tmdb.LiveKnownFor{
			{ID: 5, MediaType: "person", Name: "skip"},
			{ID: 6, MediaType: "tv", Name: "Série", FirstAirDate: "20"},
		}},
	}}
	s := withFake(t, f)
	got, err := s.TmdbPeople(1, "huppert")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d", len(got))
	}
	p := got[0]
	if p.TmdbID != 10 || p.Name != "Isabelle Huppert" || p.ProfilePath != "/ih.jpg" || p.Department != "Acting" {
		t.Errorf("person = %+v", p)
	}
	if len(p.KnownFor) != 3 {
		t.Fatalf("knownFor = %+v, want 3", p.KnownFor)
	}
	want := []string{"La Pianiste/2001/movie", "Dix pour cent/2015/tv", "Sans date/nil/movie"}
	for i, k := range p.KnownFor {
		y := "nil"
		if k.Year != nil {
			y = fmt.Sprint(*k.Year)
		}
		if s := k.Title + "/" + y + "/" + k.MediaType; s != want[i] {
			t.Errorf("knownFor[%d] = %s, want %s", i, s, want[i])
		}
	}
	if n := got[1]; n.ProfilePath != "" || n.Department != "" || n.KnownFor == nil || len(n.KnownFor) != 0 {
		t.Errorf("empty person = %+v", n)
	}
	if o := got[2].KnownFor; len(o) != 1 || o[0].Title != "Série" || o[0].Year != nil {
		t.Errorf("odd knownFor = %+v", o)
	}
}

func TestTmdbPeopleInYourList(t *testing.T) {
	f := &fakeTmdb{
		people: []tmdb.LivePersonSearchResult{{ID: 1, Name: "First"}, {ID: 2, Name: "Second"}, {ID: 3, Name: "Third"}, {ID: 4, Name: "Fourth"}},
		credits: map[int]tmdb.LiveCreditRefs{
			// Movie 100 as actor and director counts once; tv 100 is another work.
			1: {Cast: []tmdb.LiveCreditRef{ref("movie", 100), ref("tv", 100)}, Crew: []tmdb.LiveCreditRef{ref("movie", 100), ref("movie", 999)}},
			// Crew only; movie 300 is deleted from the list, movie 400 is another user's.
			2: {Crew: []tmdb.LiveCreditRef{ref("movie", 200), ref("tv", 201), ref("movie", 300), ref("movie", 400)}},
			3: {Cast: []tmdb.LiveCreditRef{ref("movie", 200)}},
		},
		failFor: map[int]bool{3: true},
	}
	s := withFake(t, f)
	addWatched(t, s, 1, entity.MOVIE, 100, false)
	addWatched(t, s, 1, entity.SHOW, 100, false)
	addWatched(t, s, 1, entity.MOVIE, 200, false)
	addWatched(t, s, 1, entity.SHOW, 201, false)
	addWatched(t, s, 1, entity.MOVIE, 300, true)
	addWatched(t, s, 2, entity.MOVIE, 400, false)

	got, err := s.TmdbPeople(1, "who")
	if err != nil {
		t.Fatal(err)
	}
	var parts []string
	for _, p := range got {
		parts = append(parts, fmt.Sprintf("%s=%d", p.Name, p.InYourList))
	}
	// Ties keep TMDB's order; the failed fetch (Third) is 0, not an error.
	if s := strings.Join(parts, ","); s != "First=2,Second=2,Third=0,Fourth=0" {
		t.Errorf("got %s", s)
	}

	// Per user: user 2 has only movie 400.
	got, _ = s.TmdbPeople(2, "who")
	if got[0].Name != "Second" || got[0].InYourList != 1 || got[1].Name != "First" || got[1].InYourList != 0 {
		t.Errorf("user 2: %+v", got)
	}
}

func TestTmdbPeopleSortAndCap(t *testing.T) {
	f := &fakeTmdb{credits: map[int]tmdb.LiveCreditRefs{}}
	for i := 1; i <= 12; i++ {
		f.people = append(f.people, tmdb.LivePersonSearchResult{ID: i, Name: fmt.Sprint("P", i)})
	}
	s := withFake(t, f)
	addWatched(t, s, 1, entity.MOVIE, 1, false)
	addWatched(t, s, 1, entity.MOVIE, 2, false)
	f.credits[5] = tmdb.LiveCreditRefs{Cast: []tmdb.LiveCreditRef{ref("movie", 1)}}
	f.credits[7] = tmdb.LiveCreditRefs{Cast: []tmdb.LiveCreditRef{ref("movie", 1), ref("movie", 2)}}
	// Beyond the 8th candidate: never looked at.
	f.credits[10] = tmdb.LiveCreditRefs{Cast: []tmdb.LiveCreditRef{ref("movie", 1), ref("movie", 2)}}

	got, err := s.TmdbPeople(1, "pp")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, p := range got {
		ids = append(ids, fmt.Sprint(p.TmdbID))
	}
	if s := strings.Join(ids, ","); s != "7,5,1,2,3,4,6,8" {
		t.Errorf("order = %s", s)
	}
	if f.maxInFlight > maxCreditsInFlight || f.maxInFlight < 2 {
		t.Errorf("max in flight = %d, want 2..%d", f.maxInFlight, maxCreditsInFlight)
	}
}

func TestTmdbPeopleEmptyAndFailedSearch(t *testing.T) {
	s := withFake(t, &fakeTmdb{})
	got, err := s.TmdbPeople(1, "nobody")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("no results: %#v %v", got, err)
	}
	s = withFake(t, &fakeTmdb{searchErr: errors.New("down")})
	if _, err := s.TmdbPeople(1, "nobody"); !errors.Is(err, errTmdbUnavailable) {
		t.Errorf("failed search: %v", err)
	}
}

func TestTmdbPeopleHTTPContract(t *testing.T) {
	f := &fakeTmdb{people: []tmdb.LivePersonSearchResult{
		{ID: 10, Name: "A", KnownFor: []tmdb.LiveKnownFor{{MediaType: "movie", Title: "M"}}},
	}}
	e := testEngineWith(withFake(t, f))

	for _, q := range []string{"", "?q=", "?q=a"} {
		if w := do(e, "GET", "/api/live/tmdb-people"+q, ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
			t.Errorf("%q: %d %s", q, w.Code, w.Body)
		}
	}
	w := do(e, "GET", "/api/live/tmdb-people?q=ab", "")
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || w.Code != 200 || len(list) != 1 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if k := keysOf(list[0]); k != "department,inYourList,knownFor,name,profilePath,tmdbId" {
		t.Errorf("keys = %s", k)
	}
	if list[0]["profilePath"] != "" || list[0]["department"] != "" || list[0]["inYourList"] != float64(0) {
		t.Errorf("defaults = %v", list[0])
	}
	kf := list[0]["knownFor"].([]any)[0].(map[string]any)
	if k := keysOf(kf); k != "mediaType,title,year" || kf["year"] != nil {
		t.Errorf("knownFor = %v", kf)
	}

	f.searchErr = errors.New("down")
	if w := do(e, "GET", "/api/live/tmdb-people?q=ab", ""); w.Code != 502 {
		t.Errorf("failed search: %d %s", w.Code, w.Body)
	}
}
