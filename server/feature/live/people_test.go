package live

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
)

func ip(n int) *int { return &n }

func cr(name, role string, tmdb *int) domain.LiveCreditRequest {
	return domain.LiveCreditRequest{Name: name, Role: role, TmdbPersonID: tmdb}
}

func createWith(t *testing.T, s *Service, userId uint, title, date string, cs ...domain.LiveCreditRequest) domain.LiveShow {
	t.Helper()
	show, err := s.Create(userId, domain.LiveShowRequest{Title: title, Date: date, Credits: &cs})
	if err != nil {
		t.Fatalf("Create(%q): %v", title, err)
	}
	return show
}

func creditStr(cs []domain.LiveCredit) string {
	var parts []string
	for _, c := range cs {
		tm := "-"
		if c.TmdbPersonID != nil {
			tm = fmt.Sprint(*c.TmdbPersonID)
		}
		parts = append(parts, fmt.Sprintf("%s/%s/%s", c.Name, c.Role, tm))
	}
	return strings.Join(parts, ",")
}

func wantValidation(t *testing.T, err error, msg string) {
	t.Helper()
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("want *ValidationError, got %v", err)
	}
	if !strings.Contains(verr.Msg, msg) {
		t.Errorf("message %q does not mention %q", verr.Msg, msg)
	}
}

func TestPeopleTableNames(t *testing.T) {
	s := newTestService(t)
	for _, name := range []string{"live_people", "live_credits"} {
		if !s.db.Migrator().HasTable(name) {
			t.Errorf("table %s was not created", name)
		}
	}
	if !s.db.Migrator().HasTable(&entity.LivePerson{}) || !s.db.Migrator().HasTable(&entity.LiveCredit{}) {
		t.Error("entities do not map to the created tables")
	}
	if !s.db.Migrator().HasIndex(&entity.LivePerson{}, "idx_live_people_user_key") {
		t.Error("unique index on (user_id, name_key) missing")
	}
}

func TestCreditsCreateReplaceKeep(t *testing.T) {
	s := newTestService(t)

	// No credits key: none, as an empty slice.
	plain := mustCreate(t, s, 1, "Plain", "2026-11-05T20:00:00Z")
	if plain.Credits == nil || len(plain.Credits) != 0 {
		t.Fatalf("credits = %#v, want empty non-nil", plain.Credits)
	}

	show := createWith(t, s, 1, "A", "2026-11-05T20:00:00Z",
		cr("  Jean   Dupont ", " Mise   en scène ", nil),
		cr("Élise Martin", "Interprète", ip(42)),
		cr("Paul", "Interprète", nil),
	)
	want := "Jean Dupont/Mise en scène/-,Élise Martin/Interprète/42,Paul/Interprète/-"
	if got := creditStr(show.Credits); got != want {
		t.Fatalf("created credits = %s, want %s", got, want)
	}
	got, _ := s.Get(1, show.ID)
	if creditStr(got.Credits) != want {
		t.Errorf("Get credits = %s", creditStr(got.Credits))
	}

	// Absent (nil) on update: unchanged.
	upd, err := s.Update(1, show.ID, domain.LiveShowRequest{Title: "A2", Date: "2026-11-05T20:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if creditStr(upd.Credits) != want {
		t.Errorf("credits changed by an update without them: %s", creditStr(upd.Credits))
	}

	// Present: full replace, new order.
	cs := []domain.LiveCreditRequest{cr("Paul", "Interprète", nil), cr("jean dupont", "Texte", nil)}
	upd, err = s.Update(1, show.ID, domain.LiveShowRequest{Title: "A2", Date: "2026-11-05T20:00:00Z", Credits: &cs})
	if err != nil {
		t.Fatal(err)
	}
	if got := creditStr(upd.Credits); got != "Paul/Interprète/-,Jean Dupont/Texte/-" {
		t.Errorf("replaced credits = %s", got)
	}

	// Present and empty: cleared.
	empty := []domain.LiveCreditRequest{}
	upd, err = s.Update(1, show.ID, domain.LiveShowRequest{Title: "A2", Date: "2026-11-05T20:00:00Z", Credits: &empty})
	if err != nil {
		t.Fatal(err)
	}
	if upd.Credits == nil || len(upd.Credits) != 0 {
		t.Errorf("credits = %#v, want []", upd.Credits)
	}

	// List carries credits too.
	createWith(t, s, 1, "B", "2026-11-06T20:00:00Z", cr("Paul", "Musique", nil))
	shows, _ := s.List(1)
	for _, sh := range shows {
		if sh.Credits == nil {
			t.Errorf("%s: nil credits in list", sh.Title)
		}
		if sh.Title == "B" && creditStr(sh.Credits) != "Paul/Musique/-" {
			t.Errorf("B credits = %s", creditStr(sh.Credits))
		}
	}
}

func TestCreditsDedupAndResolution(t *testing.T) {
	s := newTestService(t)
	a := createWith(t, s, 1, "A", "2026-11-05T20:00:00Z",
		cr("Jean Dupont", "Interprète", nil),
		cr("JEAN  dupont", "Interprète", nil), // same person, same role: collapsed
		cr("Jean Dupont", "Mise en scène", nil),
	)
	if got := creditStr(a.Credits); got != "Jean Dupont/Interprète/-,Jean Dupont/Mise en scène/-" {
		t.Fatalf("credits = %s", got)
	}
	jean := a.Credits[0].PersonID
	if a.Credits[1].PersonID != jean {
		t.Error("same name key must be the same person")
	}

	// Found by name key, no TMDB id yet: attached; stored name kept.
	b := createWith(t, s, 1, "B", "2026-11-06T20:00:00Z", cr("jean dupont", "Interprète", ip(7)))
	if b.Credits[0].PersonID != jean || b.Credits[0].Name != "Jean Dupont" || *b.Credits[0].TmdbPersonID != 7 {
		t.Errorf("attach tmdb by name: %+v", b.Credits[0])
	}

	// TMDB id wins over the name: another spelling still reuses the person,
	// and does not rename them.
	c := createWith(t, s, 1, "C", "2026-11-07T20:00:00Z", cr("J. Dupont", "Texte", ip(7)))
	if c.Credits[0].PersonID != jean || c.Credits[0].Name != "Jean Dupont" {
		t.Errorf("reuse by tmdb: %+v", c.Credits[0])
	}

	// Name only, person already linked: reused as is.
	d := createWith(t, s, 1, "D", "2026-11-08T20:00:00Z", cr("Jean Dupont", "Texte", nil))
	if d.Credits[0].PersonID != jean || *d.Credits[0].TmdbPersonID != 7 {
		t.Errorf("reuse by name: %+v", d.Credits[0])
	}

	// Accents are identity: "Elise" is not "Élise".
	e := createWith(t, s, 1, "E", "2026-11-09T20:00:00Z", cr("Élise", "Interprète", nil), cr("Elise", "Interprète", nil))
	if e.Credits[0].PersonID == e.Credits[1].PersonID {
		t.Error("Élise and Elise must be two people")
	}

	// Same name, different TMDB person: refused, nothing written.
	cs := []domain.LiveCreditRequest{cr("Paul", "Musique", nil), cr("Jean Dupont", "Interprète", ip(8))}
	_, err := s.Create(1, domain.LiveShowRequest{Title: "F", Date: "2026-11-10T20:00:00Z", Credits: &cs})
	wantValidation(t, err, `another artist named "Jean Dupont" is already linked to a different TMDB person`)
	var n int64
	s.db.Model(&entity.LiveShow{}).Where("title = ?", "F").Count(&n)
	if n != 0 {
		t.Error("show written despite the credit conflict")
	}
	s.db.Model(&entity.LivePerson{}).Where("name = ?", "Paul").Count(&n)
	if n != 0 {
		t.Error("person created despite the rollback")
	}

	// Same on update: show unchanged.
	_, err = s.Update(1, a.ID, domain.LiveShowRequest{Title: "A-changed", Date: "2026-11-05T20:00:00Z", Credits: &cs})
	wantValidation(t, err, "different TMDB person")
	got, _ := s.Get(1, a.ID)
	if got.Title != "A" || len(got.Credits) != 2 {
		t.Errorf("show altered by a failed update: %+v", got)
	}
}

func TestCreditsValidation(t *testing.T) {
	s := newTestService(t)
	tooMany := make([]domain.LiveCreditRequest, 101)
	for i := range tooMany {
		tooMany[i] = cr(fmt.Sprint("P", i), "R", nil)
	}
	tests := []struct {
		name string
		cs   []domain.LiveCreditRequest
		msg  string
	}{
		{"blank name", []domain.LiveCreditRequest{cr("  ", "R", nil)}, "credits[0]: name is required"},
		{"blank role", []domain.LiveCreditRequest{cr("X", "", nil), cr("Y", "", nil)}, "credits[0]: role is required"},
		{"long name", []domain.LiveCreditRequest{cr("ok", "R", nil), cr(strings.Repeat("é", 201), "R", nil)}, "credits[1]: name must be at most 200"},
		{"long role", []domain.LiveCreditRequest{cr("X", strings.Repeat("a", 101), nil)}, "role must be at most 100"},
		{"zero tmdb", []domain.LiveCreditRequest{cr("X", "R", ip(0))}, "tmdbPersonId"},
		{"negative tmdb", []domain.LiveCreditRequest{cr("X", "R", ip(-3))}, "tmdbPersonId"},
		{"too many", tooMany, "at most 100 credits"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.Create(1, domain.LiveShowRequest{Title: "T", Date: "2026-11-05T20:00:00Z", Credits: &tt.cs})
			wantValidation(t, err, tt.msg)
		})
	}
	// Limits count characters.
	ok := []domain.LiveCreditRequest{cr(strings.Repeat("é", 200), strings.Repeat("é", 100), nil)}
	if _, err := s.Create(1, domain.LiveShowRequest{Title: "T", Date: "2026-11-05T20:00:00Z", Credits: &ok}); err != nil {
		t.Errorf("at the limits: %v", err)
	}
	exactly := tooMany[:100]
	if _, err := s.Create(1, domain.LiveShowRequest{Title: "T", Date: "2026-11-05T20:00:00Z", Credits: &exactly}); err != nil {
		t.Errorf("100 credits: %v", err)
	}
}

func TestPeopleScopedToUser(t *testing.T) {
	s := newTestService(t)
	a := createWith(t, s, 1, "A", "2026-11-05T20:00:00Z", cr("Jean Dupont", "Interprète", ip(7)))
	pa := a.Credits[0].PersonID

	// B uses the same name and TMDB id: gets their own person.
	b := createWith(t, s, 2, "B", "2026-11-05T20:00:00Z", cr("Jean Dupont", "Texte", ip(7)))
	if b.Credits[0].PersonID == pa {
		t.Fatal("user 2 reused user 1's person")
	}
	// And a different TMDB id is no conflict across users.
	createWith(t, s, 2, "B2", "2026-11-05T20:00:00Z", cr("Paul", "Texte", ip(9)))
	createWith(t, s, 1, "A2", "2026-11-05T20:00:00Z", cr("Paul", "Texte", ip(10)))

	if _, err := s.Person(2, pa); !errors.Is(err, ErrPersonNotFound) {
		t.Errorf("Person by other user: %v", err)
	}
	if _, err := s.UpdatePerson(2, pa, domain.LivePersonUpdateRequest{Name: "x"}); !errors.Is(err, ErrPersonNotFound) {
		t.Errorf("UpdatePerson by other user: %v", err)
	}
	people, _ := s.People(2, "")
	if len(people) != 2 {
		t.Errorf("user 2 people = %+v", people)
	}
	for _, p := range people {
		if p.ID == pa {
			t.Error("user 1's person listed for user 2")
		}
		if p.Name == "Jean Dupont" && (p.TmdbPersonID == nil || *p.TmdbPersonID != 7 || p.ShowCount != 1) {
			t.Errorf("summary = %+v", p)
		}
	}
	d, _ := s.PersonByTmdb(2, 7)
	if d == nil || d.ID == pa || len(d.Roles) != 1 || d.Roles[0].Role != "Texte" {
		t.Errorf("by-tmdb for user 2 = %+v", d)
	}
	if d, _ := s.PersonByTmdb(2, 10); d != nil {
		t.Error("by-tmdb leaked user 1's person")
	}
}

func TestPeopleCountsAndSearch(t *testing.T) {
	s := newTestService(t)
	a := createWith(t, s, 1, "A", "2026-11-05T20:00:00Z",
		cr("Élise Martin", "Interprète", nil), cr("Élise Martin", "Mise en scène", nil), cr("bob", "Texte", nil))
	createWith(t, s, 1, "B", "2026-11-06T20:00:00Z", cr("Élise Martin", "Interprète", nil), cr("Alice", "Texte", nil))
	gone := createWith(t, s, 1, "C", "2026-11-07T20:00:00Z", cr("Ghost", "Texte", nil), cr("Alice", "Musique", nil))
	if err := s.Delete(1, gone.ID); err != nil {
		t.Fatal(err)
	}
	_ = a

	people, err := s.People(1, "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range people {
		got = append(got, fmt.Sprintf("%s:%d", p.Name, p.ShowCount))
	}
	// Two roles in one show count once; the deleted show counts for nobody,
	// and Ghost (only there) is hidden. Ties by name, ignoring case.
	if s := strings.Join(got, ","); s != "Élise Martin:2,Alice:1,bob:1" {
		t.Errorf("people = %s", s)
	}

	for q, want := range map[string]string{
		"elise":  "Élise Martin",
		"ÉLISE ": "Élise Martin",
		"MART":   "Élise Martin",
		"li":     "Élise Martin,Alice",
		"ghost":  "",
		"zzz":    "",
	} {
		res, err := s.People(1, q)
		if err != nil {
			t.Fatal(err)
		}
		if res == nil {
			t.Errorf("q=%q: nil result", q)
		}
		var names []string
		for _, p := range res {
			names = append(names, p.Name)
		}
		if strings.Join(names, ",") != want {
			t.Errorf("q=%q: %v, want %s", q, names, want)
		}
	}

	empty, _ := newTestService(t).People(1, "")
	if empty == nil {
		t.Error("no people: want [], got nil")
	}
}

func TestPeopleSearchCappedAt20(t *testing.T) {
	s := newTestService(t)
	cs := make([]domain.LiveCreditRequest, 30)
	for i := range cs {
		cs[i] = cr(fmt.Sprintf("Artiste %02d", i), "Interprète", nil)
	}
	createWith(t, s, 1, "A", "2026-11-05T20:00:00Z", cs...)
	all, _ := s.People(1, "")
	found, _ := s.People(1, "artiste")
	if len(all) != 30 || len(found) != 20 || found[0].Name != "Artiste 00" {
		t.Errorf("all=%d found=%d first=%v", len(all), len(found), found)
	}
}

func TestPersonDetail(t *testing.T) {
	s := newTestService(t)
	createWith(t, s, 1, "Old", "2025-01-01T20:00:00Z", cr("Élise", "Interprète", nil))
	createWith(t, s, 1, "Mid", "2026-01-01T20:00:00Z", cr("Élise", "Interprète", nil), cr("Élise", "Chorégraphie", nil))
	newest := createWith(t, s, 1, "New", "2026-06-01T20:00:00Z",
		cr("Paul", "Texte", nil), cr("Élise", "Mise en scène", nil), cr("Élise", "Interprète", nil))
	gone := createWith(t, s, 1, "Gone", "2026-07-01T20:00:00Z", cr("Élise", "Interprète", nil))
	_ = s.Delete(1, gone.ID)

	id := newest.Credits[1].PersonID
	d, err := s.Person(1, id)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range d.Roles {
		var titles []string
		for _, sh := range r.Shows {
			titles = append(titles, sh.Title)
		}
		got = append(got, r.Role+"="+strings.Join(titles, "+"))
	}
	// Most shows first, then role name; shows newest first, deleted left out.
	want := "Interprète=New+Mid+Old,Chorégraphie=Mid,Mise en scène=New"
	if g := strings.Join(got, ","); g != want {
		t.Errorf("roles = %s, want %s", g, want)
	}
	// Full shows, credits included.
	if c := d.Roles[0].Shows[0].Credits; len(c) != 3 || c[0].Name != "Paul" {
		t.Errorf("show credits in detail = %+v", c)
	}
	if d.Roles[0].Shows[0].State == "" {
		t.Error("state missing in detail shows")
	}

	if _, err := s.Person(1, 999); !errors.Is(err, ErrPersonNotFound) {
		t.Errorf("missing person: %v", err)
	}
}

func TestPersonByTmdb(t *testing.T) {
	s := newTestService(t)
	if d, err := s.PersonByTmdb(1, 7); err != nil || d != nil {
		t.Fatalf("none: %+v %v", d, err)
	}
	show := createWith(t, s, 1, "A", "2026-11-05T20:00:00Z", cr("Jean", "Interprète", ip(7)))
	d, err := s.PersonByTmdb(1, 7)
	if err != nil || d == nil || d.Name != "Jean" || len(d.Roles) != 1 || d.Roles[0].Shows[0].ID != show.ID {
		t.Fatalf("found: %+v %v", d, err)
	}
	// Only deleted shows left: null again.
	_ = s.Delete(1, show.ID)
	if d, _ := s.PersonByTmdb(1, 7); d != nil {
		t.Errorf("deleted shows only: %+v", d)
	}
}

func TestUpdatePerson(t *testing.T) {
	s := newTestService(t)
	a := createWith(t, s, 1, "A", "2026-11-05T20:00:00Z",
		cr("Jean", "Interprète", nil), cr("Paul", "Texte", ip(9)))
	jean := a.Credits[0].PersonID

	p, err := s.UpdatePerson(1, jean, domain.LivePersonUpdateRequest{Name: "  Jean   Dupont ", TmdbPersonID: ip(7)})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != jean || p.Name != "Jean Dupont" || *p.TmdbPersonID != 7 {
		t.Errorf("updated = %+v", p)
	}
	// The new name key is the identity now.
	b := createWith(t, s, 1, "B", "2026-11-06T20:00:00Z", cr("jean dupont", "Texte", nil))
	if b.Credits[0].PersonID != jean {
		t.Error("renamed person not found by the new name")
	}
	// Case-only rename of oneself is fine.
	if _, err := s.UpdatePerson(1, jean, domain.LivePersonUpdateRequest{Name: "JEAN DUPONT", TmdbPersonID: ip(7)}); err != nil {
		t.Errorf("self rename: %v", err)
	}

	_, err = s.UpdatePerson(1, jean, domain.LivePersonUpdateRequest{Name: "paul"})
	wantValidation(t, err, "an artist with that name already exists")
	_, err = s.UpdatePerson(1, jean, domain.LivePersonUpdateRequest{Name: "Jean", TmdbPersonID: ip(9)})
	wantValidation(t, err, "TMDB")
	_, err = s.UpdatePerson(1, jean, domain.LivePersonUpdateRequest{Name: " "})
	wantValidation(t, err, "name is required")
	_, err = s.UpdatePerson(1, jean, domain.LivePersonUpdateRequest{Name: "x", TmdbPersonID: ip(0)})
	wantValidation(t, err, "tmdbPersonId")

	// Null unlinks.
	p, err = s.UpdatePerson(1, jean, domain.LivePersonUpdateRequest{Name: "Jean Dupont"})
	if err != nil || p.TmdbPersonID != nil {
		t.Fatalf("unlink: %+v %v", p, err)
	}
	got, _ := s.Person(1, jean)
	if got.TmdbPersonID != nil || got.Name != "Jean Dupont" {
		t.Errorf("unlink not stored: %+v", got)
	}
}

func TestSearchFold(t *testing.T) {
	for in, want := range map[string]string{
		"Élise  Martin": "elise martin",
		"Cœur":          "coeur",
		"FRANÇOIS":      "francois",
	} {
		if got := searchFold(in); got != want {
			t.Errorf("searchFold(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPeopleHTTPContract(t *testing.T) {
	e := testEngine(t)

	if w := do(e, "GET", "/api/live/people", ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty people: %d %s", w.Code, w.Body)
	}
	if w := do(e, "GET", "/api/live/people/by-tmdb/7", ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"person":null}` {
		t.Fatalf("by-tmdb none: %d %s", w.Code, w.Body)
	}

	w := do(e, "POST", "/api/live/shows",
		`{"title":"A","date":"2026-11-05T20:00:00Z","credits":[{"name":"Élise","role":"Interprète","tmdbPersonId":7},{"name":"Paul","role":"Texte","tmdbPersonId":null},{"name":"Zoé","role":"Musique"}]}`)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var show map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &show)
	credits := show["credits"].([]any)
	if len(credits) != 3 {
		t.Fatalf("credits = %v", credits)
	}
	if k := keysOf(credits[0].(map[string]any)); k != "name,personId,role,tmdbPersonId" {
		t.Errorf("credit keys = %s", k)
	}
	if credits[1].(map[string]any)["tmdbPersonId"] != nil {
		t.Errorf("null tmdb should be null: %v", credits[1])
	}
	pid := int(credits[0].(map[string]any)["personId"].(float64))

	// credits: null keeps them.
	if w := do(e, "PUT", "/api/live/shows/1", `{"title":"A","date":"2026-11-05T20:00:00Z","credits":null}`); w.Code != 200 ||
		!strings.Contains(w.Body.String(), `"Paul"`) {
		t.Errorf("put null credits: %d %s", w.Code, w.Body)
	}

	w = do(e, "GET", "/api/live/people", "")
	var list []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 3 || keysOf(list[0]) != "id,name,showCount,tmdbPersonId" {
		t.Errorf("people = %s", w.Body)
	}
	if w := do(e, "GET", "/api/live/people?q=elise", ""); !strings.Contains(w.Body.String(), "Élise") || strings.Contains(w.Body.String(), "Paul") {
		t.Errorf("search: %s", w.Body)
	}

	w = do(e, "GET", fmt.Sprintf("/api/live/people/%d", pid), "")
	var detail map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &detail)
	if w.Code != 200 || keysOf(detail) != "id,name,roles,tmdbPersonId" {
		t.Fatalf("detail: %d %s", w.Code, w.Body)
	}
	role := detail["roles"].([]any)[0].(map[string]any)
	if keysOf(role) != "role,shows" {
		t.Errorf("role keys = %s", keysOf(role))
	}

	w = do(e, "GET", "/api/live/people/by-tmdb/7", "")
	var byTmdb map[string]map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &byTmdb)
	if w.Code != 200 || byTmdb["person"]["name"] != "Élise" {
		t.Errorf("by-tmdb found: %d %s", w.Code, w.Body)
	}

	w = do(e, "PUT", fmt.Sprintf("/api/live/people/%d", pid), `{"name":"Élise M.","tmdbPersonId":null}`)
	var upd map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &upd)
	if w.Code != 200 || keysOf(upd) != "id,name,tmdbPersonId" || upd["tmdbPersonId"] != nil {
		t.Errorf("put person: %d %s", w.Code, w.Body)
	}

	for _, tc := range []struct {
		method, path, body string
		code               int
	}{
		{"GET", "/api/live/people/0", "", 400},
		{"GET", "/api/live/people/abc", "", 400},
		{"GET", "/api/live/people/999", "", 404},
		{"GET", "/api/live/people/by-tmdb/0", "", 400},
		{"GET", "/api/live/people/by-tmdb/x", "", 400},
		{"PUT", "/api/live/people/999", `{"name":"x"}`, 404},
		{"PUT", fmt.Sprintf("/api/live/people/%d", pid), `{"name":"paul"}`, 400},
		{"PUT", fmt.Sprintf("/api/live/people/%d", pid), `nope`, 400},
		{"POST", "/api/live/shows", `{"title":"B","date":"2026-11-05T20:00:00Z","credits":[{"name":"Élise M.","role":"x","tmdbPersonId":8}]}`, 201},
		{"POST", "/api/live/shows", `{"title":"B","date":"2026-11-05T20:00:00Z","credits":[{"name":"élise m.","role":"x","tmdbPersonId":9}]}`, 400},
		{"POST", "/api/live/shows", `{"title":"B","date":"2026-11-05T20:00:00Z","credits":[{"name":"","role":"x"}]}`, 400},
	} {
		if w := do(e, tc.method, tc.path, tc.body); w.Code != tc.code {
			t.Errorf("%s %s %s: got %d, want %d (%s)", tc.method, tc.path, tc.body, w.Code, tc.code, w.Body)
		}
	}
}

func keysOf(m map[string]any) string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}
