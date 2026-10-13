package live

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
)

func TestSouvenirTableNames(t *testing.T) {
	s := newTestService(t)
	for name, model := range map[string]any{
		"live_friends":      &entity.LiveFriend{},
		"live_show_friends": &entity.LiveShowFriend{},
		"live_photos":       &entity.LivePhoto{},
	} {
		if !s.db.Migrator().HasTable(name) {
			t.Errorf("table %s was not created", name)
		}
		stmt := s.db.Model(model).Statement
		if err := stmt.Parse(model); err != nil {
			t.Fatal(err)
		}
		if stmt.Schema.Table != name {
			t.Errorf("%T maps to %q, want %q", model, stmt.Schema.Table, name)
		}
	}
	for _, col := range []string{"rating", "comment"} {
		if !s.db.Migrator().HasColumn(&entity.LiveShow{}, col) {
			t.Errorf("live_shows.%s missing", col)
		}
	}
}

func patch(t *testing.T, s *Service, userId, id uint, body string) (domain.LiveShow, error) {
	t.Helper()
	m, err := decodeSouvenirBody([]byte(body))
	if err != nil {
		return domain.LiveShow{}, err
	}
	return s.UpdateSouvenir(userId, id, m)
}

func mustPatch(t *testing.T, s *Service, userId, id uint, body string) domain.LiveShow {
	t.Helper()
	show, err := patch(t, s, userId, id, body)
	if err != nil {
		t.Fatalf("patch %s: %v", body, err)
	}
	return show
}

func friendNames(fs []domain.LiveFriend) string {
	var n []string
	for _, f := range fs {
		n = append(n, f.Name)
	}
	return strings.Join(n, ",")
}

func ratingStr(r *float64) string {
	if r == nil {
		return "nil"
	}
	return fmt.Sprint(*r)
}

// Each key is applied only when present; null clears.
func TestSouvenirPartial(t *testing.T) {
	s := newTestService(t)
	show := mustCreate(t, s, 1, "A", "2026-10-01T20:00:00Z")
	if show.Rating != nil || show.Comment != "" || show.Friends == nil || show.Photos == nil {
		t.Fatalf("new show: %+v", show)
	}

	type want struct{ rating, comment, friends string }
	check := func(got domain.LiveShow, w want) {
		t.Helper()
		if ratingStr(got.Rating) != w.rating || got.Comment != w.comment || friendNames(got.Friends) != w.friends {
			t.Errorf("got rating=%s comment=%q friends=%s, want %+v",
				ratingStr(got.Rating), got.Comment, friendNames(got.Friends), w)
		}
		again, _ := s.Get(1, show.ID)
		if ratingStr(again.Rating) != w.rating || again.Comment != w.comment || friendNames(again.Friends) != w.friends {
			t.Errorf("not persisted: %+v", again)
		}
	}

	check(mustPatch(t, s, 1, show.ID, `{"rating":7.25,"comment":"  Bien\n\nvu  ","friends":["Anne","Bob"]}`),
		want{"7.3", "Bien\n\nvu", "Anne,Bob"})
	check(mustPatch(t, s, 1, show.ID, `{"rating":8}`), want{"8", "Bien\n\nvu", "Anne,Bob"})
	check(mustPatch(t, s, 1, show.ID, `{"comment":"x"}`), want{"8", "x", "Anne,Bob"})
	check(mustPatch(t, s, 1, show.ID, `{"friends":["Bob"]}`), want{"8", "x", "Bob"})
	check(mustPatch(t, s, 1, show.ID, `{}`), want{"8", "x", "Bob"})
	check(mustPatch(t, s, 1, show.ID, `{"rating":null}`), want{"nil", "x", "Bob"})
	check(mustPatch(t, s, 1, show.ID, `{"rating":6}`), want{"6", "x", "Bob"})
	check(mustPatch(t, s, 1, show.ID, `{"rating":0}`), want{"nil", "x", "Bob"})
	check(mustPatch(t, s, 1, show.ID, `{"comment":null,"friends":null}`), want{"nil", "", ""})
	check(mustPatch(t, s, 1, show.ID, `{"friends":["Anne"],"comment":"y"}`), want{"nil", "y", "Anne"})
	check(mustPatch(t, s, 1, show.ID, `{"friends":[]}`), want{"nil", "y", ""})

	for in, out := range map[string]string{"10": "10", "9.96": "10", "0.04": "nil", "0.05": "0.1", "3.14": "3.1"} {
		got := mustPatch(t, s, 1, show.ID, `{"rating":`+in+`}`)
		if ratingStr(got.Rating) != out {
			t.Errorf("rating %s -> %s, want %s", in, ratingStr(got.Rating), out)
		}
	}
}

func TestSouvenirRejects(t *testing.T) {
	s := newTestService(t)
	show := mustCreate(t, s, 1, "A", "2026-10-01T20:00:00Z")
	mustPatch(t, s, 1, show.ID, `{"rating":5,"comment":"c","friends":["Anne"]}`)

	long := func(n int) string { return strings.Repeat("é", n) }
	many := make([]string, 31)
	for i := range many {
		many[i] = fmt.Sprintf("%q", fmt.Sprint("F", i))
	}
	for _, tc := range []struct{ body, msg string }{
		{`{"rating":-1}`, "rating must be between 0 and 10"},
		{`{"rating":10.01}`, "rating must be between 0 and 10"},
		{`{"rating":"8"}`, "rating must be a number or null"},
		{`{"comment":3}`, "comment must be a string"},
		{`{"comment":"` + long(10001) + `"}`, "comment must be at most 10000 characters"},
		{`{"friends":"Anne"}`, "friends must be a list of names"},
		{`{"friends":["Anne",1]}`, "friends must be a list of names"},
		{`{"friends":[` + strings.Join(many, ",") + `]}`, "at most 30 friends"},
		{`{"friends":["Anne","   "]}`, "friends[1]: name is required"},
		{`{"friends":["` + long(101) + `"]}`, "friends[0]: name must be at most 100 characters"},
		// A valid key does not get applied when another is rejected.
		{`{"rating":9,"comment":"` + long(10001) + `"}`, "comment must be at most 10000 characters"},
		{`[]`, "invalid request body"},
		{`null`, "invalid request body"},
	} {
		_, err := patch(t, s, 1, show.ID, tc.body)
		var verr *ValidationError
		if !errors.As(err, &verr) || verr.Msg != tc.msg {
			t.Errorf("%.60s: err = %v, want %q", tc.body, err, tc.msg)
		}
	}
	// Limits themselves are accepted.
	ok := make([]string, 30)
	for i := range ok {
		ok[i] = fmt.Sprintf("%q", fmt.Sprint("F", i))
	}
	mustPatch(t, s, 1, show.ID, `{"comment":"`+long(10000)+`","friends":[`+strings.Join(ok, ",")+`]}`)
	mustPatch(t, s, 1, show.ID, `{"comment":"c","friends":["`+long(100)+`"]}`)
	mustPatch(t, s, 1, show.ID, `{"friends":["Anne"]}`)

	got, _ := s.Get(1, show.ID)
	if ratingStr(got.Rating) != "5" || got.Comment != "c" || friendNames(got.Friends) != "Anne" {
		t.Errorf("show altered by rejected patches: %+v", got)
	}
}

func TestSouvenirFriendsResolve(t *testing.T) {
	s := newTestService(t)
	a := mustCreate(t, s, 1, "A", "2026-10-01T20:00:00Z")
	b := mustCreate(t, s, 1, "B", "2026-10-02T20:00:00Z")

	// Order kept, duplicates by name key collapsed (accents count).
	got := mustPatch(t, s, 1, a.ID, `{"friends":[" Élise  Martin ","bob","élise martin","Bob","Elise Martin"]}`)
	if friendNames(got.Friends) != "Élise Martin,bob,Elise Martin" {
		t.Errorf("friends = %s", friendNames(got.Friends))
	}
	if n := countRows(t, s, &entity.LiveFriend{}); n != 3 {
		t.Errorf("live_friends = %d, want 3", n)
	}
	bob := got.Friends[1].ID

	// Reused by key, stored spelling kept; new one created.
	got = mustPatch(t, s, 1, b.ID, `{"friends":["Chloé","BOB"]}`)
	if friendNames(got.Friends) != "Chloé,bob" || got.Friends[1].ID != bob {
		t.Errorf("friends = %+v", got.Friends)
	}
	if n := countRows(t, s, &entity.LiveFriend{}); n != 4 {
		t.Errorf("live_friends = %d, want 4", n)
	}

	// Full replace, in the new order.
	got = mustPatch(t, s, 1, a.ID, `{"friends":["bob","Zoé"]}`)
	if friendNames(got.Friends) != "bob,Zoé" {
		t.Errorf("replace: %s", friendNames(got.Friends))
	}
	if n := countRows(t, s, &entity.LiveShowFriend{}); n != 4 {
		t.Errorf("live_show_friends = %d, want 4", n)
	}

	// Another user's "Bob" is their own friend.
	c := mustCreate(t, s, 2, "C", "2026-10-02T20:00:00Z")
	got = mustPatch(t, s, 2, c.ID, `{"friends":["Bob"]}`)
	if got.Friends[0].ID == bob || got.Friends[0].Name != "Bob" {
		t.Errorf("user 2 got user 1's friend: %+v", got.Friends)
	}

	// List shows the friends too.
	list, _ := s.List(1)
	if friendNames(list[0].Friends) != "bob,Zoé" || friendNames(list[1].Friends) != "Chloé,bob" {
		t.Errorf("List friends: %+v / %+v", list[0].Friends, list[1].Friends)
	}
}

func TestSouvenirScoping(t *testing.T) {
	s := newTestService(t)
	show := mustCreate(t, s, 1, "A", "2026-10-01T20:00:00Z")
	if _, err := patch(t, s, 2, show.ID, `{"rating":5,"friends":["X"]}`); !errors.Is(err, ErrNotFound) {
		t.Errorf("other user: %v", err)
	}
	if _, err := patch(t, s, 1, 999, `{"rating":5}`); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing show: %v", err)
	}
	if n := countRows(t, s, &entity.LiveFriend{}); n != 0 {
		t.Errorf("friends created for a refused patch: %d", n)
	}
	got, _ := s.Get(1, show.ID)
	if got.Rating != nil {
		t.Errorf("rating set by another user")
	}
	e2 := testEngineAs(s, 2)
	if w := do(e2, "PATCH", fmt.Sprintf("/api/live/shows/%d/souvenir", show.ID), `{"rating":5}`); w.Code != 404 {
		t.Errorf("HTTP other user: %d %s", w.Code, w.Body)
	}
	if w := do(e2, "GET", "/api/live/friends", ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("friends of user 2: %d %s", w.Code, w.Body)
	}
}

func TestFriendsList(t *testing.T) {
	s := newTestService(t)
	mk := func(title, friends string) domain.LiveShow {
		sh := mustCreate(t, s, 1, title, "2026-10-01T20:00:00Z")
		mustPatch(t, s, 1, sh.ID, `{"friends":`+friends+`}`)
		return sh
	}
	mk("A", `["Élise","bob"]`)
	mk("B", `["bob","Chloé"]`)
	mk("C", `["Chloé","alice"]`)
	d := mk("D", `["Zoé","bob"]`)
	if err := s.Delete(1, d.ID); err != nil {
		t.Fatal(err)
	}
	mustPatch(t, s, 2, mustCreate(t, s, 2, "E", "2026-10-01T20:00:00Z").ID, `{"friends":["Other"]}`)

	names := func(l []domain.LiveFriendSummary) string {
		var n []string
		for _, f := range l {
			n = append(n, fmt.Sprintf("%s:%d", f.Name, f.ShowCount))
		}
		return strings.Join(n, ",")
	}
	all, err := s.Friends(1, "")
	if err != nil {
		t.Fatal(err)
	}
	// Zoé only came to a deleted show; bob's deleted show is not counted.
	if got := names(all); got != "bob:2,Chloé:2,alice:1,Élise:1" {
		t.Errorf("Friends = %s", got)
	}
	for q, want := range map[string]string{
		"elise": "Élise:1", "CHLOE": "Chloé:2", "o": "bob:2,Chloé:2", "  ": "bob:2,Chloé:2,alice:1,Élise:1", "zoe": "",
	} {
		got, _ := s.Friends(1, q)
		if names(got) != want {
			t.Errorf("q=%q: %s, want %s", q, names(got), want)
		}
	}

	// At most 20 results with a search.
	var many []string
	for i := 0; i < 25; i++ {
		many = append(many, fmt.Sprintf("%q", fmt.Sprintf("Ami %02d", i)))
	}
	mk("F", "["+strings.Join(many, ",")+"]")
	if got, _ := s.Friends(1, "ami"); len(got) != 20 || got[0].Name != "Ami 00" {
		t.Errorf("search limit: %d results", len(got))
	}
	if got, _ := s.Friends(1, ""); len(got) != 29 {
		t.Errorf("no limit without q: %d", len(got))
	}

	w := do(testEngineWith(s), "GET", "/api/live/friends?q=bo", "")
	var body []map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != 200 || len(body) != 1 || body[0]["name"] != "bob" || body[0]["showCount"] != float64(2) || body[0]["id"] == nil || len(body[0]) != 3 {
		t.Errorf("HTTP friends: %d %s", w.Code, w.Body)
	}
}

// PUT is the show's details only: souvenir keys in its body are ignored.
func TestPutIgnoresSouvenir(t *testing.T) {
	s := newTestService(t)
	e := testEngineWith(s)
	show := mustCreate(t, s, 1, "A", "2026-10-01T20:00:00Z")
	mustPatch(t, s, 1, show.ID, `{"rating":7.5,"comment":"Super","friends":["Anne"]}`)
	if _, err := s.AddPhoto(1, show.ID, strings.NewReader(string(pngOf(red))), false, ""); err != nil {
		t.Fatal(err)
	}

	w := do(e, "PUT", fmt.Sprintf("/api/live/shows/%d", show.ID),
		`{"title":"B","date":"2026-10-01T20:00:00Z","rating":null,"comment":"","friends":[],"photos":[]}`)
	if w.Code != 200 {
		t.Fatalf("PUT: %d %s", w.Code, w.Body)
	}
	var got domain.LiveShow
	json.Unmarshal(w.Body.Bytes(), &got)
	again, _ := s.Get(1, show.ID)
	for _, sh := range []domain.LiveShow{got, again} {
		if sh.Title != "B" || ratingStr(sh.Rating) != "7.5" || sh.Comment != "Super" ||
			friendNames(sh.Friends) != "Anne" || len(sh.Photos) != 1 {
			t.Errorf("souvenir lost by PUT: %+v", sh)
		}
	}

	// Through the service too.
	if _, err := s.Update(1, show.ID, domain.LiveShowRequest{Title: "C", Date: "2026-10-01T20:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if again, _ = s.Get(1, show.ID); ratingStr(again.Rating) != "7.5" || again.Comment != "Super" {
		t.Errorf("Update cleared the souvenir: %+v", again)
	}
}

func TestSouvenirHTTPContract(t *testing.T) {
	s := newTestService(t)
	e := testEngineWith(s)
	w := do(e, "POST", "/api/live/shows", `{"title":"A","date":"2026-10-01T20:00:00Z"}`)
	var obj map[string]any
	json.Unmarshal(w.Body.Bytes(), &obj)
	if r, ok := obj["rating"]; !ok || r != nil {
		t.Errorf("rating = %#v (present %v), want null", r, ok)
	}
	if obj["comment"] != "" {
		t.Errorf("comment = %#v", obj["comment"])
	}
	for _, k := range []string{"friends", "photos"} {
		if v, ok := obj[k].([]any); !ok || len(v) != 0 {
			t.Errorf("%s = %#v, want []", k, obj[k])
		}
	}
	id := int(obj["id"].(float64))
	path := fmt.Sprintf("/api/live/shows/%d/souvenir", id)

	w = do(e, "PATCH", path, `{"rating":8.5,"comment":"ok","friends":["Anne"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH: %d %s", w.Code, w.Body)
	}
	obj = nil
	json.Unmarshal(w.Body.Bytes(), &obj)
	f, _ := obj["friends"].([]any)
	if obj["rating"] != 8.5 || obj["comment"] != "ok" || len(f) != 1 || obj["title"] != "A" {
		t.Errorf("PATCH response: %s", w.Body)
	} else if fr := f[0].(map[string]any); fr["name"] != "Anne" || fr["id"] == nil || len(fr) != 2 {
		t.Errorf("friend = %#v", fr)
	}

	for _, tc := range []struct {
		path, body string
		code       int
	}{
		{path, `{"rating":11}`, 400},
		{path, `not json`, 400},
		{path, `{"rating":null}`, 200},
		{"/api/live/shows/999/souvenir", `{}`, 404},
		{"/api/live/shows/abc/souvenir", `{}`, 400},
	} {
		if w := do(e, "PATCH", tc.path, tc.body); w.Code != tc.code {
			t.Errorf("PATCH %s %s: %d, want %d (%s)", tc.path, tc.body, w.Code, tc.code, w.Body)
		}
	}
	w = do(e, "GET", fmt.Sprintf("/api/live/shows/%d", id), "")
	if !strings.Contains(w.Body.String(), `"rating":null`) {
		t.Errorf("cleared rating not null: %s", w.Body)
	}
}
