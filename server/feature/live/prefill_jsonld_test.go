package live

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sbondCo/Watcharr/domain"
)

// prefillFixture runs a saved real page through the parser, as if fetched
// from pageURL.
func prefillFixture(t *testing.T, file, pageURL string) domain.LivePrefill {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	u, _ := url.Parse(pageURL)
	return buildPrefill(u, parsePage(f))
}

// One Event per performance, at the end of the body; og:title carries the
// credits and the site name.
func TestPrefillFixtureLesTheatres(t *testing.T) {
	got := prefillFixture(t, "lestheatres-fete-des-meres.html",
		"https://www.lestheatres.net/fr/a/6102-fete-des-meres-diptyque-familial")
	if got.Title != "FÊTE DES MÈRES, DIPTYQUE FAMILIAL" {
		t.Errorf("title: %q", got.Title)
	}
	if got.Venue != "Bernardines" {
		t.Errorf("venue: %q", got.Venue)
	}
	// og's synopsis, not the JSON-LD credits.
	if !strings.HasPrefix(got.Description, "Entre une fête des mères") {
		t.Errorf("description: %q", got.Description)
	}
	if got.PosterURL != "https://d2d4te11holpqx.cloudfront.net/picture/12964/og/fete-des-meres-diptyque-familial---adele-royne-vincent-gardet-compagnie-midiminuit-les-theatres-o-saison-26o27.jpg" {
		t.Errorf("poster: %q", got.PosterURL)
	}
	want := []string{
		"2026-10-06T20:00:00+02:00",
		"2026-10-07T19:00:00+02:00",
		"2026-10-08T20:00:00+02:00",
		"2026-10-09T20:00:00+02:00",
		"2026-10-10T20:00:00+02:00",
	}
	if !reflect.DeepEqual(got.Performances, want) {
		t.Errorf("performances: %q", got.Performances)
	}
	if got.DateRange != nil {
		t.Errorf("dateRange: %+v", *got.DateRange)
	}
}

// One Event in the head, days without a time, a longer synopsis than og, and
// an empty og:image.
func TestPrefillFixtureJoliette(t *testing.T) {
	got := prefillFixture(t, "joliette-batir.html",
		"https://www.theatrejoliette.fr/programmation/26-27/batir")
	if got.Title != "Bâtir" {
		t.Errorf("title: %q", got.Title)
	}
	if got.Venue != "Théâtre Joliette" {
		t.Errorf("venue: %q", got.Venue)
	}
	if !strings.HasPrefix(got.Description, "Politique de logement") ||
		!strings.HasSuffix(got.Description, "habitant·es de ces cités.") ||
		strings.Contains(got.Description, "\n") {
		t.Errorf("description: %q", got.Description)
	}
	if got.PosterURL != "https://www.theatrejoliette.fr/media/pages/programmation/26-27/batir/d6f910d085-1782291014/batir-2-900x.jpg" {
		t.Errorf("poster: %q", got.PosterURL)
	}
	if got.Performances == nil || len(got.Performances) != 0 {
		t.Errorf("performances: %#v", got.Performances)
	}
	if got.DateRange == nil || *got.DateRange != (domain.LiveDateRange{Start: "2027-04-07", End: "2027-04-09"}) {
		t.Errorf("dateRange: %+v", got.DateRange)
	}
}

// No JSON-LD at all: Open Graph only.
func TestPrefillFixtureLaCriee(t *testing.T) {
	got := prefillFixture(t, "lacriee-boat-people.html",
		"https://theatre-lacriee.com/programmation/evenements/boat-people")
	want := domain.LivePrefill{
		URL:          "https://theatre-lacriee.com/programmation/evenements/boat-people",
		Title:        "Boat People",
		PosterURL:    "https://theatre-lacriee.com/storage/2026/04/17/boat-people-covercablain-10.png",
		Performances: []string{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("\n got %+v\nwant %+v", got, want)
	}
}

func TestStripSiteSuffix(t *testing.T) {
	for _, tc := range []struct{ title, site, want string }{
		{"Bâtir | Théâtre Joliette", "Théâtre Joliette", "Bâtir"},
		{"Bâtir - Théâtre Joliette", "Théâtre Joliette", "Bâtir"},
		{"Bâtir – Théâtre Joliette", "Théâtre Joliette", "Bâtir"},
		{"Bâtir — Théâtre Joliette", "Théâtre Joliette", "Bâtir"},
		{"Bâtir • Théâtre Joliette", "Théâtre Joliette", "Bâtir"},
		{"Bâtir · Théâtre Joliette", "Théâtre Joliette", "Bâtir"},
		// Case-insensitive match, the rest of the segment goes too.
		{"Show - Credits | LES THÉÂTRES • Saison 26•27", "Les Théâtres", "Show - Credits"},
		// Twice over.
		{"Show | Site | Site", "Site", "Show"},
		// No site name: nothing to anchor on.
		{"Show | Something", "", "Show | Something"},
		// The site name in the middle is part of the title.
		{"Une nuit au Théâtre Joliette - Création", "Théâtre Joliette", "Une nuit au Théâtre Joliette - Création"},
		{"Show | Théâtre Joliette - Création", "Théâtre Joliette", "Show"},
		// Without a separator before it, it is not a suffix.
		{"Théâtre Joliette", "Théâtre Joliette", "Théâtre Joliette"},
		{"Molière à Théâtre Joliette", "Théâtre Joliette", "Molière à Théâtre Joliette"},
		// Keeps the original case.
		{"FÊTE DES MÈRES", "Les Théâtres", "FÊTE DES MÈRES"},
	} {
		if got := stripSiteSuffix(tc.title, tc.site); got != tc.want {
			t.Errorf("stripSiteSuffix(%q, %q) = %q, want %q", tc.title, tc.site, got, tc.want)
		}
	}
}

func ldPage(blocks ...string) string {
	var b strings.Builder
	b.WriteString(`<html><head><meta property="og:site_name" content="Site"></head><body>`)
	for _, s := range blocks {
		b.WriteString(`<script type="application/ld+json">` + s + `</script>`)
	}
	b.WriteString(`</body></html>`)
	return b.String()
}

func prefillOf(page string) domain.LivePrefill {
	u, _ := url.Parse("https://theatre.example/show")
	return buildPrefill(u, parsePage(strings.NewReader(page)))
}

func TestJSONLDShapes(t *testing.T) {
	for _, tc := range []struct {
		name, page, title string
	}{
		{"object", ldPage(`{"@type":"Event","name":"A"}`), "A"},
		{"array", ldPage(`[{"@type":"WebPage","name":"no"},{"@type":"Event","name":"A"}]`), "A"},
		{"graph", ldPage(`{"@context":"https://schema.org","@graph":[{"@type":"Organization","name":"no"},{"@type":"Event","name":"A"}]}`), "A"},
		{"type array", ldPage(`{"@type":["Thing","TheaterEvent"],"name":"A"}`), "A"},
		{"subtype", ldPage(`{"@type":"DanceEvent","name":"A"}`), "A"},
		{"festival", ldPage(`{"@type":"Festival","name":"A"}`), "A"},
		{"type IRI", ldPage(`{"@type":"http://schema.org/Event","name":"A"}`), "A"},
		{"escaped name", ldPage(`{"@type":"Event","name":"L&#039;Avare &amp; co"}`), "L'Avare & co"},
		{"broken block skipped", ldPage(`{"@type":"Event","name":"no",}`, `{"@type":"Event","name":"A"}`), "A"},
		{"not an event", ldPage(`{"@type":"Place","name":"no"}`), ""},
		{"in the head", `<html><head><script type="application/ld+json">{"@type":"Event","name":"A"}</script></head></html>`, "A"},
		{"other script ignored", `<script type="text/javascript">{"@type":"Event","name":"no"}</script>`, ""},
		{"json garbage", ldPage(`null`, `42`, `"x"`, `[[]]`, `{"@type":{"x":1},"name":["a"]}`), ""},
	} {
		if got := prefillOf(tc.page); got.Title != tc.title {
			t.Errorf("%s: title %q, want %q", tc.name, got.Title, tc.title)
		}
	}
}

func TestJSONLDFields(t *testing.T) {
	// og:title loses to the event name, og:site_name to the place, and the
	// longer description wins.
	page := `<html><head>
<meta property="og:title" content="Long og title | Site">
<meta property="og:description" content="Short.">
<meta property="og:site_name" content="Site">
</head><body><script type="application/ld+json">{"@type":"Event","name":"A",
"description":"A much longer synopsis.",
"location":[{"@type":"VirtualLocation"},{"type":"Place","name":"Grande salle"}],
"image":[{"@type":"ImageObject","url":"/big.jpg"}]}</script></body></html>`
	got := prefillOf(page)
	if got.Title != "A" || got.Venue != "Grande salle" || got.Description != "A much longer synopsis." {
		t.Errorf("got %+v", got)
	}
	// The JSON-LD image only when there is no og or twitter one.
	if got.PosterURL != "https://theatre.example/big.jpg" {
		t.Errorf("poster: %q", got.PosterURL)
	}
	got = prefillOf(strings.Replace(page, `<meta property="og:site_name"`,
		`<meta property="og:image" content="/og.jpg"><meta property="og:site_name"`, 1))
	if got.PosterURL != "https://theatre.example/og.jpg" {
		t.Errorf("og:image must win: %q", got.PosterURL)
	}

	// Without JSON-LD the site name is the venue, and the suffix goes.
	got = prefillOf(`<head><title>Show | Site</title><meta property="og:site_name" content="Site"><meta name="description" content="Meta."></head>`)
	if got.Title != "Show" || got.Venue != "Site" || got.Description != "Meta." {
		t.Errorf("got %+v", got)
	}
}

func TestJSONLDDates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		events []string
		perfs  []string
		rng    *domain.LiveDateRange
	}{
		{"offset kept", []string{`"startDate":"2026-10-06T20:00:00+02:00"`}, []string{"2026-10-06T20:00:00+02:00"}, nil},
		{"UTC kept", []string{`"startDate":"2026-10-06T18:00:00Z"`}, []string{"2026-10-06T18:00:00Z"}, nil},
		{"no offset is Paris, summer", []string{`"startDate":"2026-10-06T20:00:00"`}, []string{"2026-10-06T20:00:00+02:00"}, nil},
		{"no offset is Paris, winter", []string{`"startDate":"2026-12-06T20:00"`}, []string{"2026-12-06T20:00:00+01:00"}, nil},
		{"sorted and deduplicated", []string{
			`"startDate":"2026-10-08T20:00:00+02:00"`,
			`"startDate":"2026-10-06T20:00:00+02:00"`,
			`"startDate":"2026-10-06T18:00:00Z"`, // same instant
			`"startDate":"2026-10-07T19:00"`,
		}, []string{"2026-10-06T20:00:00+02:00", "2026-10-07T19:00:00+02:00", "2026-10-08T20:00:00+02:00"}, nil},
		{"date only", []string{`"startDate":"2027-04-07","endDate":"2027-04-09"`}, []string{}, &domain.LiveDateRange{Start: "2027-04-07", End: "2027-04-09"}},
		{"date only, no end", []string{`"startDate":"2027-04-07"`}, []string{}, &domain.LiveDateRange{Start: "2027-04-07", End: "2027-04-07"}},
		{"days over several events", []string{
			`"startDate":"2027-04-10"`,
			`"startDate":"2027-04-07","endDate":"2027-04-08"`,
		}, []string{}, &domain.LiveDateRange{Start: "2027-04-07", End: "2027-04-10"}},
		{"times win over days", []string{`"startDate":"2027-04-07"`, `"startDate":"2027-04-08T20:00:00+02:00"`},
			[]string{"2027-04-08T20:00:00+02:00"}, nil},
		{"unreadable dates", []string{`"startDate":"demain"`, `"startDate":12`}, []string{}, nil},
	} {
		var blocks []string
		for _, e := range tc.events {
			blocks = append(blocks, `{"@type":"Event","name":"A",`+e+`}`)
		}
		got := prefillOf(ldPage(blocks...))
		if !reflect.DeepEqual(got.Performances, tc.perfs) || !reflect.DeepEqual(got.DateRange, tc.rng) {
			t.Errorf("%s: performances %q, dateRange %+v", tc.name, got.Performances, got.DateRange)
		}
	}
}

func TestJSONLDPerformancesCapped(t *testing.T) {
	var blocks []string
	for d := 1; d <= 28; d++ {
		for _, h := range []string{"14", "17", "20"} {
			blocks = append(blocks, `{"@type":"Event","name":"A","startDate":"2027-02-`+
				twoDigits(d)+`T`+h+`:00:00+01:00"}`)
		}
	}
	got := prefillOf(ldPage(`[` + strings.Join(blocks, ",") + `]`))
	if len(got.Performances) != maxPerformances || got.Performances[0] != "2027-02-01T14:00:00+01:00" {
		t.Errorf("%d performances, first %q", len(got.Performances), got.Performances[0])
	}
}

func twoDigits(n int) string {
	return string([]byte{byte('0' + n/10), byte('0' + n%10)})
}
