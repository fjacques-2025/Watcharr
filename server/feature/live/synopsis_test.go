package live

import (
	"net/url"
	"os"
	"strings"
	"testing"
)

func synopsisOf(t *testing.T, file, page string) string {
	t.Helper()
	f, err := os.Open("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	u, _ := url.Parse(page)
	return buildPrefill(u, parsePage(f)).Description
}

// La Criée declares no description: the synopsis comes from the page.
func TestSynopsisFromPageLaCriee(t *testing.T) {
	d := synopsisOf(t, "lacriee-boat-people.html", "https://theatre-lacriee.com/programmation/evenements/boat-people")
	for _, want := range []string{"Où sont leur", "1979, près de Niort", "Marine Bachelot Nguyen fait du théâtre"} {
		if !strings.Contains(d, want) {
			t.Errorf("missing %q in:\n%s", want, d)
		}
	}
	for _, not := range []string{"Coproduction", "rue Jobin", "billetterie", "Depuis une vingtaine"} {
		if strings.Contains(d, not) {
			t.Errorf("%q should not be in the synopsis:\n%s", not, d)
		}
	}
	t.Logf("synopsis (%d chars):\n%s", len([]rune(d)), d)
}

// Pages that declare a description keep it.
func TestSynopsisDeclaredWins(t *testing.T) {
	d := synopsisOf(t, "lestheatres-fete-des-meres.html", "https://www.lestheatres.net/fr/a/6102-fete-des-meres-diptyque-familial")
	if !strings.HasPrefix(d, "Entre une fête des mères") {
		t.Errorf("declared description lost: %.80q", d)
	}
}

func TestBodySynopsisRules(t *testing.T) {
	long := "Une famille accueille une autre famille sous son toit, et rien ne se passe comme prévu dans cette maison."
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"cookie banner skipped", []string{"Nous utilisons des cookies pour améliorer votre expérience sur notre site, acceptez-vous leur utilisation ici ?", long}, long},
		{"heading between paragraphs kept going", []string{long, "Voir plus", long + " Encore."}, long + "\n\n" + long + " Encore."},
		{"stops at production", []string{long, "Production Compagnie X, coproduction Théâtre Y et Scène nationale Z, avec le soutien de la DRAC."}, long},
		{"stops at a phone number", []string{long, "Renseignements au 04 91 54 70 54 du mardi au samedi de 12h à 18h sauf jours fériés et ponts."}, long},
		{"menus only: nothing", []string{"Accueil", "Programmation", "Billetterie"}, ""},
	}
	for _, c := range cases {
		if got := bodySynopsis(c.lines); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}
