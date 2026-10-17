package live

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// La Criée declares no description at all (its meta description says
// "null"), yet the synopsis is the first paragraphs of the page. When a page
// declares none, bodySynopsis takes its first run of prose, stopping at what
// is no longer the show's presentation: address, phone, production credits,
// press quotes.

const maxSynopsis = 2500

var (
	sentenceEnd = regexp.MustCompile(`[.?!…»"”]`)
	phone       = regexp.MustCompile(`\b0\d([ .]?\d{2}){4}\b`)
	// Lines that end the presentation once it has started.
	synopsisStop = regexp.MustCompile(`^(production|coproduction|co-production|soutien|avec le soutien|remerciements|mentions|partenaires|billetterie|tarifs?\b|dates?\b|durée|accès|adresse|«)`)
	// Not the show's text wherever they are: banners and boilerplate.
	notSynopsis = regexp.MustCompile(`cookie|javascript|navigateur|newsletter|©|tous droits|données personnelles|consentement`)
)

func bodySynopsis(lines []string) string {
	var out []string
	total := 0
	for _, l := range lines {
		low := strings.ToLower(l)
		n := utf8.RuneCountInString(l)
		if notSynopsis.MatchString(low) {
			continue
		}
		if synopsisStop.MatchString(low) || phone.MatchString(l) {
			if len(out) > 0 {
				break
			}
			continue
		}
		if !isProse(l, n) {
			// A heading or a button between paragraphs doesn't end the text;
			// a long line that isn't prose does.
			if len(out) > 0 && n >= 80 {
				break
			}
			continue
		}
		out = append(out, l)
		total += n
		if total >= maxSynopsis {
			break
		}
	}
	return strings.Join(out, "\n\n")
}

// isProse: sentences, not a menu, a heading or a list of names.
func isProse(l string, n int) bool {
	return n >= 80 && len(strings.Fields(l)) >= 12 && sentenceEnd.MatchString(l)
}
