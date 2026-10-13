package live

import (
	"net/url"
	"os"
	"strings"
	"testing"
)

// The Opéra-Odéon declares og:image:alt but no og:image: the poster is only
// in the page, named after the show.
func TestPosterFromPageOdeon(t *testing.T) {
	f, err := os.Open("testdata/odeon-la-jalousie.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	page, _ := url.Parse("https://opera-odeon.marseille.fr/programmation/la-jalousie")
	got := buildPrefill(page, parsePage(f))
	want := "https://opera-odeon.marseille.fr/sites/default/files/2026-04/la-jalousie-c.marcel-hartmann-2.png"
	if got.PosterURL != want {
		t.Errorf("poster = %q, want %q", got.PosterURL, want)
	}
	if got.Title != "LA JALOUSIE" {
		t.Errorf("title = %q", got.Title)
	}
}

func TestPosterFromPageRules(t *testing.T) {
	page, _ := url.Parse("https://theatre.example/fr/a/6102-fete-des-meres")
	cases := []struct {
		name, title, html, want string
	}{
		{"declared image wins", "Fête des mères",
			`<head><meta property="og:image" content="/declared.jpg"></head><body><img src="/fete-des-meres.jpg"></body>`,
			"https://theatre.example/declared.jpg"},
		{"slug from the URL, leading id dropped", "",
			`<body><img src="/logo.png"><img src="/up/fete-des-meres-affiche.jpg"></body>`,
			"https://theatre.example/up/fete-des-meres-affiche.jpg"},
		{"slug from the title, accents folded", "FÊTE DES MÈRES",
			`<body><img src="/x/FETE_des_meres.PNG"></body>`,
			"https://theatre.example/x/FETE_des_meres.PNG"},
		{"lazy source preferred", "",
			`<body><img src="/placeholder.gif" data-src="/fete-des-meres.jpg"></body>`,
			"https://theatre.example/fete-des-meres.jpg"},
		{"logo named after the show is skipped", "",
			`<body><img src="/fete-des-meres-logo.png"></body>`, ""},
		{"formats the saver can't store are skipped", "",
			`<body><img src="/fete-des-meres.webp"><img src="/fete-des-meres.svg"></body>`, ""},
		{"no match: no poster rather than a guess", "",
			`<body><img src="/banner.jpg"><img src="/photo-1.jpg"></body>`, ""},
	}
	for _, c := range cases {
		m := parsePage(strings.NewReader("<html>" + c.html + "</html>"))
		if got := poster(page, c.title, m, nil); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSlugFold(t *testing.T) {
	for in, want := range map[string]string{
		"LA JALOUSIE":                   "la-jalousie",
		"Fête des mères, diptyque":      "fete-des-meres-diptyque",
		"  --Bâtir--  ":                 "batir",
		"la-jalousie-c.marcel-hartmann": "la-jalousie-c-marcel-hartmann",
	} {
		if got := slugFold(in); got != want {
			t.Errorf("slugFold(%q) = %q, want %q", in, got, want)
		}
	}
}
