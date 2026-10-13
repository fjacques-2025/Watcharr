package tmdb

import (
	"encoding/json"
	"testing"
)

// The shapes TMDB sends: known_for differs between movies and TV, and
// combined credits have a crew list next to the cast.
func TestLiveStructsDecode(t *testing.T) {
	var s liveSearchPeopleResponse
	err := json.Unmarshal([]byte(`{"page":1,"results":[{"id":1,"name":"A","profile_path":null,
		"known_for_department":"Acting","known_for":[
		{"id":2,"media_type":"movie","title":"M","release_date":"2001-02-03"},
		{"id":3,"media_type":"tv","name":"T","first_air_date":"2015-10-14"}]}]}`), &s)
	if err != nil {
		t.Fatal(err)
	}
	p := s.Results[0]
	if p.ProfilePath != "" || p.KnownForDepartment != "Acting" || len(p.KnownFor) != 2 ||
		p.KnownFor[0].Title != "M" || p.KnownFor[0].ReleaseDate != "2001-02-03" ||
		p.KnownFor[1].Name != "T" || p.KnownFor[1].FirstAirDate != "2015-10-14" {
		t.Errorf("decoded %+v", p)
	}

	var c LiveCreditRefs
	err = json.Unmarshal([]byte(`{"id":1,"cast":[{"id":2,"media_type":"movie"}],"crew":[{"id":3,"media_type":"tv","job":"Director"}]}`), &c)
	if err != nil || len(c.Cast) != 1 || c.Crew[0] != (LiveCreditRef{ID: 3, MediaType: "tv"}) {
		t.Errorf("credits %+v %v", c, err)
	}
}
