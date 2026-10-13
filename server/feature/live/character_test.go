package live

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sbondCo/Watcharr/domain"
)

type domainCredit = domain.LiveCreditRequest

func chr(name, role, character string) domainCredit {
	return domain.LiveCreditRequest{Name: name, Role: role, Character: character}
}

func charShowReq(title, date string, cs []domainCredit) domain.LiveShowRequest {
	return domain.LiveShowRequest{Title: title, Date: date, Credits: &cs}
}

func charStr(cs []domain.LiveCredit) string {
	var parts []string
	for _, c := range cs {
		parts = append(parts, c.Name+"/"+c.Role+"/"+c.Character)
	}
	return strings.Join(parts, ",")
}

func TestCreditCharacters(t *testing.T) {
	s := newTestService(t)
	show := createWith(t, s, 1, "A", "2026-11-05T20:00:00Z",
		chr("Élise", "Performer", "  Lady   Macbeth "),
		chr("Paul", "Director", ""),
		chr("Zoé", "Performer", ""),
		chr("Zoé", "Performer", "Witch"),
		chr("Zoé", "Performer", "Ghost"),
	)
	if got := charStr(show.Credits); got != "Élise/Performer/Lady Macbeth,Paul/Director/,Zoé/Performer/Witch" {
		t.Errorf("create = %s", got)
	}
	got, _ := s.Get(1, show.ID)
	if c := charStr(got.Credits); c != "Élise/Performer/Lady Macbeth,Paul/Director/,Zoé/Performer/Witch" {
		t.Errorf("get = %s", c)
	}

	cs := []domainCredit{chr("Paul", "Director", "Himself")}
	upd, err := s.Update(1, show.ID, charShowReq("A", "2026-11-05T20:00:00Z", cs))
	if err != nil {
		t.Fatal(err)
	}
	if c := charStr(upd.Credits); c != "Paul/Director/Himself" {
		t.Errorf("replace = %s", c)
	}

	// Person detail carries it too.
	d, err := s.Person(1, upd.Credits[0].PersonID)
	if err != nil || d.Roles[0].Shows[0].Credits[0].Character != "Himself" {
		t.Errorf("detail: %+v %v", d, err)
	}

	_, err = s.Create(1, charShowReq("B", "2026-11-05T20:00:00Z", []domainCredit{
		chr("X", "Performer", "ok"), chr("Y", "Performer", strings.Repeat("é", maxCharacter+1)),
	}))
	wantValidation(t, err, "credits[1]: character must be at most 200 characters")
	if _, err := s.Create(1, charShowReq("B", "2026-11-05T20:00:00Z", []domainCredit{
		chr("Y", "Performer", strings.Repeat("é", maxCharacter)),
	})); err != nil {
		t.Errorf("exactly %d: %v", maxCharacter, err)
	}
}

func TestCreditCharacterHTTP(t *testing.T) {
	e := testEngine(t)
	w := do(e, "POST", "/api/live/shows",
		`{"title":"A","date":"2026-11-05T20:00:00Z","credits":[{"name":"Élise","role":"Performer","character":" Ophélie ","tmdbPersonId":7},{"name":"Paul","role":"Director"}]}`)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var show map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &show)
	credits := show["credits"].([]any)
	if c := credits[0].(map[string]any)["character"]; c != "Ophélie" {
		t.Errorf("character = %v", c)
	}
	if c, ok := credits[1].(map[string]any)["character"]; !ok || c != "" {
		t.Errorf("no character = %v (present %v)", c, ok)
	}
	w = do(e, "POST", "/api/live/shows",
		`{"title":"A","date":"2026-11-05T20:00:00Z","credits":[{"name":"Élise","role":"Performer","character":"`+strings.Repeat("x", 201)+`"}]}`)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "credits[0]: character must be at most 200 characters") {
		t.Errorf("too long: %d %s", w.Code, w.Body)
	}
	// Same credit shape on an artist's page.
	if w := do(e, "GET", "/api/live/people/by-tmdb/7", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"character":"Ophélie"`) {
		t.Errorf("by-tmdb: %d %s", w.Code, w.Body)
	}
}

func TestCharacterColumn(t *testing.T) {
	s := newTestService(t)
	if !s.db.Migrator().HasColumn("live_credits", "character_name") {
		t.Error("live_credits.character_name missing")
	}
}
