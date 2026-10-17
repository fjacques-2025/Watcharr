package live

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sbondCo/Watcharr/domain"
)

// show renders suggestions one per line, "Name: Role, Role" with a "*" when
// suggested, so a failing fixture prints a readable diff.
func show(cs []domain.LiveCreditSuggestion) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString(c.Name + ": " + strings.Join(c.Roles, ", "))
		if c.Suggested {
			b.WriteString(" *")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func pageCredits(t *testing.T, file string) []domain.LiveCreditSuggestion {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// The same path as the prefill: lines up to an "other shows" heading.
	return parseCreditLines(ownShowLines(parsePage(f).lines))
}

func checkCredits(t *testing.T, got []domain.LiveCreditSuggestion, want string) {
	t.Helper()
	want = strings.TrimLeft(want, "\n")
	if s := show(got); s != want {
		t.Errorf("credits:\n--- got\n%s--- want\n%s", s, want)
	}
}

// The user's pasted example. "Calton Mu holove" keeps its lowercase word
// (known label, see validName); Osvaldo merges his avec line into his crew
// line.
func TestCreditsConverge(t *testing.T) {
	b, err := os.ReadFile("testdata/distribution-converge.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseCredits(string(b))
	if err != nil {
		t.Fatal(err)
	}
	checkCredits(t, got, `
Converge+: Company *
Idio Chichava: Director, Choreographer *
Osvaldo Passirivo: Assistant, Directeur des répétitions, Performer *
Açucena Chemane: Performer *
Arminda Zunguza: Performer *
Calton Mu holove: Performer *
Cristina Matola: Performer *
Fernando Machaieie: Performer *
Judite Novela: Performer *
Mauro Sigauque: Performer *
Martins Tuvanji: Performer *
Nilégio Cossa: Performer *
Patrick Manuel Sitoe: Performer *
Stela Matsombe: Performer *
Vasco Sitoe: Performer *
Phayra Baloi: Lumière
Silvana Pombal: Management de tournée
`)
}

// Nothing from the menu ("Avec vous"), the header (bare names) or the
// synopsis ("Acteur, auteur et metteur en scène, Salim Djaferi aime…").
func TestCreditsJoliette(t *testing.T) {
	checkCredits(t, pageCredits(t, "joliette-batir.html"), `
Marie Alié: Writer *
Salim Djaferi: Writer, Performer, Director *
Clément Papachristou: Writer, Director *
Sasha Martelli: Performer, Collaboration artistique *
Hanna El Fakir: Documentation, Collaboration artistique, Assistanat mise en scène, Coordination artistique
Adeline Rosenstein: Dramaturgie
Janoé Vulbeau: Collaboration à la recherche
Justine Bougerol: Scénographie
Silvio Palomo: Scénographie
Laurie Fouvet: Création lumière, Direction technique
Sophie Melis: Coach mouvement
Maïa Blondeau: Création sonore
Skandar Kazan Seckar: Assistanat mise en scène, Coordination artistique
Silvia Ruth Hasenclever: Costumes
Alice Spenlé: Régie plateau, Son
Yorick Detroy: Régie plateau, Son
Nicolas Marty: Régie plateau, Son
Cora-Line Lefèvre: Accompagnement, Production, Diffusions
Rosine Louviaux: Accompagnement, Production, Diffusions
Alix Maraval: Accompagnement, Production, Diffusions
Apolline Paquet: Accompagnement, Production, Diffusions
`)
}

// "Un spectacle d’Adèle Royné" (elision glued to the name), the avec line
// ending in "et Adèle Royné", nothing from "Création Le 1er octobre 2026 -
// Théâtre d’Arles". "Production Compagnie et diffusion Laure Duqué & …" is
// dropped: "diffusion Laure Duqué" is not a name. Nothing from below "Vous
// aimerez aussi", where "Compagnie A Kan la Dériv'" belongs to another show.
func TestCreditsLesTheatres(t *testing.T) {
	checkCredits(t, pageCredits(t, "lestheatres-fete-des-meres.html"), `
Adèle Royné: Director, Writer, Performer *
Guillaume Vincent: Collaboration artistique
Vincent Gardet: Writer *
César Godefroy: Lumières
Lucie Ben Bâta: Costumes
Noa Gimenez: Collaboration à la scénographie
Léo Delorme: Régie générale, Lumière
Andréa Millerand: Régie plateau
ACME Camille Torre: Diffusion
Axel Yvon: Diffusion
Pierre-Guilhem Coste: Dessin
Joaquim Fossi: Performer *
Elsa Guedj: Performer *
Aubin Hernandez: Performer *
Florence Janas: Performer *
Cyril Metzger: Performer *
Cie MidiMinuit: Production
`)
}

// Labels alone on their line, names on the next; "Distribution" headings
// pair with nothing; "Avec l’aide à la production de la DGCA…" is a sentence.
func TestCreditsLaCriee(t *testing.T) {
	checkCredits(t, pageCredits(t, "lacriee-boat-people.html"), `
Marine Bachelot Nguyen: Writer, Director *
Clément Bigot: Performer *
Charline Grand: Performer *
Arnold Mensah: Performer *
Paul Nguyen: Performer *
Dorothée Saysombat: Performer, Écriture marionnettique *
Angélica Kiyomi Tisseyre-Sékiné: Performer *
Yen Linh Tham: Assistanat
Kim Lan Nguyen Thi: Scénographie
Julie Pareau: Création vidéo, Régie générale
Alice Gill-Kahn: Création lumière
Yohann Gabillard: Création sonore
Laure Fonvieille: Création costumes
Anna Sauvage: Régie lumière
`)
}

func TestCreditLine(t *testing.T) {
	for _, tc := range []struct {
		line  string
		roles []string
		names []string // nil: not a credit line
	}{
		{"Texte et mise en scène : Marine Bachelot Nguyen", []string{"Writer", "Director"}, []string{"Marine Bachelot Nguyen"}},
		{"conception & chorégraphie Idio Chichava", []string{"Director", "Choreographer"}, []string{"Idio Chichava"}},
		{"Mise en scène – Jean Dupont", []string{"Director"}, []string{"Jean Dupont"}},
		{"Musique originale: Jean Dupont", []string{"Music"}, []string{"Jean Dupont"}},
		{"Traduction Jean Dupont", []string{"Translation"}, []string{"Jean Dupont"}},
		{"D’après Anton Tchekhov", []string{"Writer"}, []string{"Anton Tchekhov"}},
		{"Un spectacle d’ Adèle Royné", []string{"Director"}, []string{"Adèle Royné"}},
		{"Un spectacle de Adèle Royné", []string{"Director"}, []string{"Adèle Royné"}},
		{"Distribution Jean Dupont", []string{"Performer"}, []string{"Jean Dupont"}},
		{"Coach mouvement Sophie Melis", []string{"Coach mouvement"}, []string{"Sophie Melis"}},
		{"Création lumière et direction technique Laurie Fouvet", []string{"Création lumière", "Direction technique"}, []string{"Laurie Fouvet"}},
		{"Assistanat mise en scène, coordination artistique Hanna El Fakir , Skandar Kazan Seckar",
			[]string{"Assistanat mise en scène", "Coordination artistique"}, []string{"Hanna El Fakir", "Skandar Kazan Seckar"}},
		{"Régie plateau et son Alice Spenlé en collaboration avec Yorick Detroy et Nicolas Marty",
			[]string{"Régie plateau", "Son"}, []string{"Alice Spenlé", "Yorick Detroy", "Nicolas Marty"}},
		// Parenthesised details are dropped.
		{"Avec Jean Dupont (en alternance), Marie Curie.", []string{"Performer"}, []string{"Jean Dupont", "Marie Curie"}},
		// An institution's long name among the people: kept out, the people kept.
		{"Lumière Jean Dupont et Le Grand Théâtre National de la Ville de Marseille Provence",
			[]string{"Lumière"}, []string{"Jean Dupont"}},

		{"Acteur, auteur et metteur en scène, Salim Djaferi aime explorer et tricoter les récits", nil, nil},
		{"Avec vous", nil, nil},
		{"Création Le 1er octobre 2026 - Théâtre d’Arles", nil, nil},
		{"Dates : du 19 au 21 novembre 2026", nil, nil},
		{"Distribution", nil, nil},
		{"Distribution & Production", nil, nil},
		{"De Molière", nil, nil},
		{"Salim Djaferi", nil, nil},
		{"Théâtre Joliette", nil, nil},
		{"Jeu de Paume", nil, nil},
		{"Avec l’aide à la production de la DGCA-Ministère de la Culture", nil, nil},
		// One short invalid piece rejects the whole line.
		{"Production Compagnie et diffusion Laure Duqué & Grégoire Faugeras", nil, nil},
		// Labels over 8 words.
		{"Un deux trois quatre cinq six sept huit lumière Jean Dupont", nil, nil},
	} {
		c, ok := parseCreditLine(tokenize(tc.line))
		if tc.names == nil {
			if ok {
				t.Errorf("%q: got %v %v, want no credit", tc.line, c.label.roles, c.names)
			}
			continue
		}
		if !ok || !reflect.DeepEqual(c.label.roles, tc.roles) || !reflect.DeepEqual(c.names, tc.names) {
			t.Errorf("%q: got %v %v (%v), want %v %v", tc.line, c.label.roles, c.names, ok, tc.roles, tc.names)
		}
	}
}

func TestValidName(t *testing.T) {
	for _, tc := range []struct {
		name    string
		lenient bool
		want    bool
	}{
		{"Jean Dupont", false, true},
		{"Converge+", false, true},
		{"Cora-Line Lefèvre", false, true},
		{"Élise Durand", false, true},
		{"Jean de La Fontaine", false, true},
		{"Ludwig van Beethoven", false, true},
		{"Théâtre d’Arles", false, true},
		{"Hanna El Fakir", false, true},
		{"Sacha GUITRY", false, true},
		{"vous", false, false},
		{"Jean de", false, false},
		{"Jean Dupont 2", false, false},
		{"Le 1er octobre", false, false},
		{"Jean Dupont aime", false, false},
		{"Un Deux Trois Quatre Cinq Six Sept", false, false},
		{"Jean « Dupont »", false, false},
		// Lenient: one lowercase word, from the third word, in at most 4.
		{"Calton Mu holove", true, true},
		{"Calton Mu holove", false, false},
		{"Calton holove", true, false},
		{"Calton Mu holove ha", true, false},
		{"Jean Dupont assisté de Marie", true, false},
	} {
		if got := validName(tokenize(tc.name), tc.lenient); got != tc.want {
			t.Errorf("validName(%q, %v) = %v, want %v", tc.name, tc.lenient, got, tc.want)
		}
	}
}

func TestCreditsNextLine(t *testing.T) {
	got, _ := ParseCredits(strings.Join([]string{
		"Distribution", // heading: does not take "Avec" as a name
		"Avec",
		"Clément Bigot, Charline Grand",
		"Scénographie",
		"Kim Lan Nguyen Thi",
		"Texte :",
		"Marine Bachelot Nguyen",
		// A menu: unknown label over a single word.
		"Espace technique",
		"Calendrier",
		// Not a role.
		"La saison",
		"Grand Théâtre",
		// A label followed by another credit line: no pairing.
		"Lumière",
		"Costumes Jean Dupont",
		// Pairing only to the very next line.
		"Musique",
		"",
		"Paul Nguyen",
	}, "\n"))
	checkCredits(t, got, `
Clément Bigot: Performer *
Charline Grand: Performer *
Kim Lan Nguyen Thi: Scénographie
Marine Bachelot Nguyen: Writer *
Jean Dupont: Costumes
`)
}

func TestCreditsMerge(t *testing.T) {
	got, _ := ParseCredits("Texte Élise  Durand\r\nMise en scène ELISE DURAND, Paul Nguyen\nAvec élise durand\nJeu Elise Durand\nLumière Paul Nguyen\nTexte Paul Nguyen")
	checkCredits(t, got, `
Élise Durand: Writer, Director, Performer *
Paul Nguyen: Director, Lumière, Writer *
`)
	// Accent-folded identity: "Elise Durand" merged into "Élise Durand",
	// first spelling kept; "élise durand" is not a name (lowercase).
	if got[0].Name != "Élise Durand" {
		t.Errorf("name = %q", got[0].Name)
	}
}

func TestCreditsLimits(t *testing.T) {
	long := "Avec Jean Dupont, " + strings.Repeat("Marie Curie, ", 40)
	if got, _ := ParseCredits(long); len(got) != 0 {
		t.Errorf("line over 400 characters: %v", show(got))
	}
	if _, err := ParseCredits(strings.Repeat("é", maxCreditsText)); err != nil {
		t.Errorf("at the limit: %v", err)
	}
	if _, err := ParseCredits(strings.Repeat("é", maxCreditsText+1)); err == nil {
		t.Error("over the limit: no error")
	}
	for _, text := range []string{"", "\n\n", "Bonjour"} {
		got, err := ParseCredits(text)
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("%q: %#v %v, want an empty non-nil slice", text, got, err)
		}
	}
}

func TestParseCreditsHTTP(t *testing.T) {
	e := testEngine(t)

	w := do(e, "POST", "/api/live/parse-credits", `{"text":"lumière Phayra Baloi\navec Açucena Chemane"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("parse: %d %s", w.Code, w.Body)
	}
	want := `{"credits":[{"name":"Phayra Baloi","roles":["Lumière"],"suggested":false},{"name":"Açucena Chemane","roles":["Performer"],"suggested":true}]}`
	if strings.TrimSpace(w.Body.String()) != want {
		t.Errorf("body = %s\nwant %s", w.Body, want)
	}

	for _, body := range []string{`{"text":""}`, `{}`} {
		w := do(e, "POST", "/api/live/parse-credits", body)
		if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"credits":[]}` {
			t.Errorf("%s: %d %s, want 200 {\"credits\":[]}", body, w.Code, w.Body)
		}
	}

	text, _ := json.Marshal(map[string]string{"text": strings.Repeat("a", maxCreditsText+1)})
	for _, body := range []string{string(text), `not json`, `{"text":3}`} {
		if w := do(e, "POST", "/api/live/parse-credits", body); w.Code != http.StatusBadRequest {
			t.Errorf("%.40s: %d, want 400", body, w.Code)
		}
	}
}

// The prefill response carries the page's credits.
func TestPrefillCreditsHTTP(t *testing.T) {
	e := testEngine(t)
	srv := pageServer(t, map[string]string{
		"/show": `<html><head><title>T</title><script>var a = "Avec Faux Nom";</script></head><body>
<nav><a>Accueil</a><a>Avec vous</a></nav>
<p>Texte et mise en scène&nbsp;: <strong>Marine Bachelot&nbsp;Nguyen</strong></p>
<p>Avec<br>Clément Bigot, Charline Grand</p>
<template><p>Avec Caché Template</p></template>
<svg><text>Avec Caché Svg</text></svg>
<table><tr><td>Lumière</td><td>Alice Gill-Kahn</td></tr></table>
</body></html>`,
	})
	w := do(e, "POST", "/api/live/prefill", `{"url":"`+srv.URL+`/show"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("prefill: %d %s", w.Code, w.Body)
	}
	var got struct {
		Credits []domain.LiveCreditSuggestion `json:"credits"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	checkCredits(t, got.Credits, `
Marine Bachelot Nguyen: Writer, Director *
Clément Bigot: Performer *
Charline Grand: Performer *
Alice Gill-Kahn: Lumière
`)
}

func TestOwnShowLinesStopsAtOtherShows(t *testing.T) {
	lines := []string{"Avec Jeanne Martin", "Vous aimerez aussi", "Compagnie Autre Spectacle"}
	if got := ownShowLines(lines); len(got) != 1 {
		t.Errorf("kept %q", got)
	}
	// Pasted text is never cut: it is the user's own selection.
	got, err := ParseCredits(strings.Join(lines, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("pasted text lost credits: %+v", got)
	}
	// Case and a variant heading.
	if got := ownShowLines([]string{"x", "À DÉCOUVRIR AUSSI", "y"}); len(got) != 1 {
		t.Errorf("variant heading not seen: %q", got)
	}
}
