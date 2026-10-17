package live

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sbondCo/Watcharr/domain"
)

// Credits are read from the "distribution" a theatre page prints: one credit
// per line, "Label names" or "Label" then names on the next line. They are
// suggestions the user validates, so the rules lean towards missing a credit
// rather than inventing one out of a menu or a synopsis.

const (
	maxCreditsText    = 20000
	maxCreditLine     = 400
	maxLabelWords     = 8
	maxNameWords      = 6
	maxSuggestedNames = 200
)

// The app's default roles, as the frontend names them.
const (
	rolePerformer     = "Performer"
	roleWriter        = "Writer"
	roleDirector      = "Director"
	roleChoreographer = "Choreographer"
	roleCompany       = "Company"
	roleMusic         = "Music"
	roleTranslation   = "Translation"
)

// knownLabels maps a folded label to its default role.
var knownLabels = map[string]string{
	"avec": rolePerformer, "jeu": rolePerformer, "interpretation": rolePerformer,
	"interpretes": rolePerformer, "interprete": rolePerformer, "danse": rolePerformer,
	"danseurs": rolePerformer, "danseuses": rolePerformer, "comediens": rolePerformer,
	"comediennes": rolePerformer, "distribution": rolePerformer,

	"texte": roleWriter, "textes": roleWriter, "ecriture": roleWriter,
	"auteur": roleWriter, "autrice": roleWriter, "d'apres": roleWriter, "livret": roleWriter,

	"mise en scene": roleDirector, "conception": roleDirector,
	"un spectacle de": roleDirector, "un spectacle d'": roleDirector, "direction": roleDirector,

	"choregraphie": roleChoreographer,

	"compagnie": roleCompany, "cie": roleCompany, "collectif": roleCompany,

	"musique": roleMusic, "composition": roleMusic, "musique originale": roleMusic,

	"traduction": roleTranslation, "adaptation": roleTranslation,
}

// knownLabelWords: the known labels as word lists, longest first, for
// matching at the start of a line.
var knownLabelWords = func() [][]string {
	var out [][]string
	for k := range knownLabels {
		out = append(out, strings.Fields(k))
	}
	// Longest first so "musique originale" beats "musique".
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}()

// Headings over a credits block. Alone on a line they name no one, so they
// must not pair with the next line ("Distribution" then "Avec" would make
// someone called "Avec").
var creditHeadings = map[string]bool{
	"distribution": true, "production": true, "generique": true,
	"credits": true, "equipe": true, "equipe artistique": true,
}

// A label made only of one of these is a sentence, not a role: "De Molière",
// "Par Jean Dupont".
var labelStopwords = map[string]bool{
	"de": true, "d'": true, "du": true, "des": true, "par": true, "pour": true,
	"le": true, "la": true, "les": true, "l'": true, "un": true, "une": true,
	"a": true, "au": true, "aux": true, "en": true, "et": true, "sur": true, "dans": true,
}

// roleStems: an unknown label must contain a word starting with one of these
// to count as a role. Without it any two-word line ("Dominique Bluzet",
// "Théâtre Joliette", "Presse Production" in a footer) reads as "Label
// Name". Coproduction/soutien/partenaires are deliberately absent: they list
// institutions, not people.
var roleStems = []string{
	"accessoir", "accompagn", "administrat", "arrang", "assist", "chant",
	"charge", "chef", "coach", "coiffur", "collaborat", "communicat",
	"conseil", "construct", "coordinat", "costum", "creat", "decor",
	"dessin", "diffusion", "direct", "dramat", "dramaturg", "eclairag",
	"ecritur", "graphi", "habill", "illustr", "image", "lumiere",
	"machin", "management", "maquill", "marionnet", "mouvement",
	"musici", "perruq", "photo", "plateau", "preparat", "product",
	"regard", "regie", "regisseu", "scenograph", "son", "sonor",
	"surtitr", "techni", "tournee", "video", "voix",
}

// Name particles: lowercase words a name may contain.
var nameParticles = map[string]bool{
	"de": true, "du": true, "des": true, "la": true, "le": true, "les": true,
	"d'": true, "van": true, "von": true, "da": true, "di": true, "del": true,
	"el": true, "al": true, "ben": true, "bin": true, "y": true, "e": true,
	"dos": true, "das": true,
}

var defaultRoles = map[string]bool{
	rolePerformer: true, roleWriter: true, roleDirector: true, roleChoreographer: true,
	roleCompany: true, roleMusic: true, roleTranslation: true,
}

// token is a word of a line. glued: no space before it in the text, as
// "Adèle" after "d’" in "d’Adèle".
type token struct {
	text  string
	fold  string
	glued bool
}

var (
	parenthetical = regexp.MustCompile(`\([^()]*\)`)
	// Elisions are split off so "d’Adèle" matches "un spectacle d'" and
	// "Adèle" is checked as a capitalised word.
	elision = regexp.MustCompile(`^([dDlL]['’])(.+)$`)
	// Zero-width characters some CMSs leave at the start of paragraphs.
	invisible = strings.NewReplacer("\u200b", "", "\u200c", "", "\u200d", "", "\ufeff", "", "\u00ad", "")
	padPunct  = strings.NewReplacer(":", " : ", ",", " , ", ";", " , ", "&", " & ")
)

func foldWord(s string) string {
	return strings.ReplaceAll(searchFold(s), "’", "'")
}

func tokenize(line string) []token {
	line = parenthetical.ReplaceAllString(line, " ")
	var out []token
	for _, w := range strings.Fields(padPunct.Replace(line)) {
		if m := elision.FindStringSubmatch(w); m != nil {
			out = append(out, token{text: m[1], fold: foldWord(m[1])})
			out = append(out, token{text: m[2], fold: foldWord(m[2]), glued: true})
			continue
		}
		out = append(out, token{text: w, fold: foldWord(w)})
	}
	return out
}

func joinTokens(ts []token) string {
	var b strings.Builder
	for i, t := range ts {
		if i > 0 && !t.glued {
			b.WriteByte(' ')
		}
		b.WriteString(t.text)
	}
	return b.String()
}

func wordCount(ts []token) int {
	n := 0
	for _, t := range ts {
		if !t.glued {
			n++
		}
	}
	return n
}

func startsUpper(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsUpper(r)
}

func hasDigit(ts []token) bool {
	for _, t := range ts {
		if strings.IndexFunc(t.text, unicode.IsDigit) >= 0 {
			return true
		}
	}
	return false
}

func isSeparator(t token) bool {
	switch t.text {
	case ":", "-", "–", "—":
		return true
	}
	return false
}

func isConnector(t token) bool {
	switch t.fold {
	case "et", "&", ",", "and":
		return true
	}
	return false
}

// matchKnownLabel returns the length in tokens of the longest known label at
// ts[i:], 0 if none.
func matchKnownLabel(ts []token, i int) int {
	for _, words := range knownLabelWords {
		if i+len(words) > len(ts) {
			continue
		}
		ok := true
		for j, w := range words {
			if ts[i+j].fold != w {
				ok = false
				break
			}
		}
		if ok {
			return len(words)
		}
	}
	return 0
}

// knownLabelPrefix consumes known labels joined by connectors at the start
// of the line ("Texte et mise en scène"), then a separator. Returns where the
// names start, 0 if the line does not start with a known label.
func knownLabelPrefix(ts []token) int {
	i := 0
	for {
		n := matchKnownLabel(ts, i)
		if n == 0 {
			break
		}
		i += n
		// A connector belongs to the label only if another label follows.
		if i < len(ts) && isConnector(ts[i]) && matchKnownLabel(ts, i+1) > 0 {
			i++
			continue
		}
		break
	}
	for i > 0 && i < len(ts) && isSeparator(ts[i]) {
		i++
	}
	return i
}

// labelInfo is a label split into its roles.
type labelInfo struct {
	roles []string
	// Every part is a known label: names may then be looser (see validName).
	allKnown bool
	// Some part is a heading that only counts with names on the same line.
	heading bool
}

// parseLabel splits a label on " et ", " & ", "," and maps each part.
// ok is false when the label does not look like a role.
func parseLabel(ts []token) (labelInfo, bool) {
	for len(ts) > 0 && isSeparator(ts[len(ts)-1]) {
		ts = ts[:len(ts)-1]
	}
	// Ending on "&" or "et": the next word belongs to the label
	// ("Distribution & Production"), so it is not the first name.
	if len(ts) == 0 || isConnector(ts[len(ts)-1]) || wordCount(ts) > maxLabelWords || hasDigit(ts) {
		return labelInfo{}, false
	}
	info := labelInfo{allKnown: true}
	anyKnown, stem := false, false
	var part []token
	flush := func() bool {
		if len(part) == 0 {
			return true
		}
		var folds []string
		for _, t := range part {
			folds = append(folds, t.fold)
		}
		key := strings.Join(folds, " ")
		// Elisions are glued in the text: "un spectacle d'" folds the same
		// with or without the space.
		key = strings.ReplaceAll(key, "' ", "'")
		if labelStopwords[key] {
			return false
		}
		if creditHeadings[key] {
			info.heading = true
		}
		role, known := knownLabels[key]
		if known {
			anyKnown = true
		} else {
			// "Avec l’aide à la production de la DGCA": a label does not
			// end on "de la", a sentence running into a name does.
			if labelStopwords[folds[len(folds)-1]] {
				return false
			}
			info.allKnown = false
			role = freeRole(joinTokens(part))
			for _, f := range folds {
				for _, s := range roleStems {
					if strings.HasPrefix(f, s) {
						stem = true
					}
				}
			}
		}
		if role != "" {
			info.roles = appendUnique(info.roles, role)
		}
		part = part[:0]
		return true
	}
	for _, t := range ts {
		if isConnector(t) {
			if !flush() {
				return labelInfo{}, false
			}
			continue
		}
		if isSeparator(t) {
			continue
		}
		part = append(part, t)
	}
	if !flush() || len(info.roles) == 0 || !(anyKnown || stem) {
		return labelInfo{}, false
	}
	return info, true
}

// freeRole: the label as the user would type it, "lumière" → "Lumière".
func freeRole(s string) string {
	s = strings.Trim(collapseSpaces(s), " :-–—.,;")
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	return truncateRunes(string(unicode.ToUpper(r))+s[size:], maxRole)
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// validName checks a candidate name. lenient (the label is a known one, so
// the line is surely a credit) tolerates ONE lowercase word from the third
// word on, in a name of at most 4 words: "Calton Mu holove" from a real cast
// list. Prose ("Jean Dupont aime explorer les récits…") still fails on
// length or on a second lowercase word, and a name must start capitalised,
// which rejects "Avec vous" or "avec le soutien de…".
func validName(ts []token, lenient bool) bool {
	n := wordCount(ts)
	if n == 0 || n > maxNameWords || hasDigit(ts) || !startsUpper(ts[0].text) {
		return false
	}
	if nameParticles[ts[len(ts)-1].fold] {
		return false
	}
	lower := 0
	word := -1
	for _, t := range ts {
		if !t.glued {
			word++
		}
		for _, r := range t.text {
			if !unicode.IsLetter(r) && !unicode.IsMark(r) && !strings.ContainsRune("-'’.+", r) {
				return false
			}
		}
		if startsUpper(t.text) || nameParticles[t.fold] {
			continue
		}
		lower++
		if !lenient || lower > 1 || word < 2 || n > 4 {
			return false
		}
	}
	return true
}

// parseNames splits a names part on ",", " et ", " & ", " and ", "en
// collaboration avec". A piece that is not a name rejects the line, unless
// every such piece is longer than a name could be: that is an
// institution's long name among the people, and only the people are kept.
func parseNames(ts []token, lenient bool) []string {
	var names []string
	badShort := false
	var piece []token
	flush := func() {
		// "Marty." at the end of a sentence; "J." initials keep their dot.
		if len(piece) > 0 {
			last := piece[len(piece)-1]
			if utf8.RuneCountInString(last.text) > 2 && strings.HasSuffix(last.text, ".") {
				last.text = strings.TrimSuffix(last.text, ".")
				piece[len(piece)-1] = last
			}
		}
		if len(piece) == 0 {
			return
		}
		if validName(piece, lenient) {
			names = append(names, joinTokens(piece))
		} else if wordCount(piece) <= maxNameWords {
			badShort = true
		}
		piece = nil
	}
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		if isConnector(t) {
			flush()
			continue
		}
		if t.fold == "en" && i+2 < len(ts) && ts[i+1].fold == "collaboration" && ts[i+2].fold == "avec" {
			flush()
			i += 2
			continue
		}
		piece = append(piece, t)
	}
	flush()
	if badShort {
		return nil
	}
	return names
}

// lineCredit is one line read as "label names".
type lineCredit struct {
	label labelInfo
	names []string
}

// parseCreditLine tries, in order: known labels at the start ("Texte et mise
// en scène : …"), an explicit separator ("Label : names", "Label – names"),
// then a split before the first capitalised word after the first one
// ("Coach mouvement Sophie Melis").
func parseCreditLine(ts []token) (lineCredit, bool) {
	try := func(label, names []token) (lineCredit, bool) {
		// "Distribution & Production": a known label then a connector is
		// a label going on, not names.
		if len(names) == 0 || isConnector(names[0]) {
			return lineCredit{}, false
		}
		info, ok := parseLabel(label)
		if !ok {
			return lineCredit{}, false
		}
		ns := parseNames(names, info.allKnown)
		if len(ns) == 0 {
			return lineCredit{}, false
		}
		return lineCredit{info, ns}, true
	}
	if i := knownLabelPrefix(ts); i > 0 {
		if c, ok := try(ts[:i], ts[i:]); ok {
			return c, true
		}
	}
	sep := -1
	for i, t := range ts {
		if t.text == ":" {
			sep = i
			break
		}
	}
	if sep < 0 {
		for i, t := range ts {
			if isSeparator(t) {
				sep = i
				break
			}
		}
	}
	if sep > 0 {
		if c, ok := try(ts[:sep], ts[sep+1:]); ok {
			return c, true
		}
	}
	for k := 1; k < len(ts); k++ {
		if startsUpper(ts[k].text) {
			return try(ts[:k], ts[k:])
		}
	}
	return lineCredit{}, false
}

// labelOnly reports whether a line is a label waiting for its names on the
// next line (La Criée prints "Scénographie" then "Kim Lan Nguyen Thi").
func labelOnly(ts []token) (labelInfo, bool) {
	if len(ts) == 0 || wordCount(ts) > maxLabelWords {
		return labelInfo{}, false
	}
	if knownLabelPrefix(ts) != len(ts) {
		// Not made of known labels: then no capitalised word after the
		// first, or it is a line of names rather than a label.
		for _, t := range ts[1:] {
			if startsUpper(t.text) {
				return labelInfo{}, false
			}
		}
	}
	info, ok := parseLabel(ts)
	if !ok || info.heading {
		return labelInfo{}, false
	}
	return info, true
}

// parseCreditLines reads credits from lines of text, in order of first
// appearance, one entry per person.
func parseCreditLines(lines []string) []domain.LiveCreditSuggestion {
	out := []domain.LiveCreditSuggestion{}
	index := map[string]int{}
	add := func(c lineCredit) {
		for _, name := range c.names {
			key := foldWord(name)
			i, seen := index[key]
			if !seen {
				if len(out) >= maxSuggestedNames {
					continue
				}
				i = len(out)
				index[key] = i
				out = append(out, domain.LiveCreditSuggestion{Name: name, Roles: []string{}})
			}
			for _, r := range c.label.roles {
				out[i].Roles = appendUnique(out[i].Roles, r)
			}
		}
	}

	toks := make([][]token, len(lines))
	for i, l := range lines {
		l = collapseSpaces(invisible.Replace(l))
		if utf8.RuneCountInString(l) <= maxCreditLine {
			toks[i] = tokenize(l)
		}
	}
	for i := 0; i < len(toks); i++ {
		ts := toks[i]
		if len(ts) == 0 {
			continue
		}
		if c, ok := parseCreditLine(ts); ok {
			add(c)
			continue
		}
		label, ok := labelOnly(ts)
		if !ok || i+1 >= len(toks) {
			continue
		}
		next := toks[i+1]
		// The next line must be names alone, not another credit or label.
		if len(next) == 0 {
			continue
		}
		if _, ok := parseCreditLine(next); ok {
			continue
		}
		if _, ok := labelOnly(next); ok {
			continue
		}
		names := parseNames(next, label.allKnown)
		if len(names) == 0 {
			continue
		}
		// An unknown label over a single word is too often a menu entry
		// over the next one ("Espace technique" / "Calendrier").
		if !label.allKnown && !anyMultiWord(names) {
			continue
		}
		add(lineCredit{label, names})
		i++
	}
	for i := range out {
		for _, r := range out[i].Roles {
			if defaultRoles[r] {
				out[i].Suggested = true
				break
			}
		}
	}
	return out
}

func anyMultiWord(names []string) bool {
	for _, n := range names {
		if strings.Contains(n, " ") {
			return true
		}
	}
	return false
}

// ParseCredits reads credits from text the user pasted.
func ParseCredits(text string) ([]domain.LiveCreditSuggestion, error) {
	if utf8.RuneCountInString(text) > maxCreditsText {
		return nil, invalid("text must be at most %d characters", maxCreditsText)
	}
	// Blank lines are kept: a label must not pair with names past one.
	text = strings.NewReplacer("\r\n", "\n", "\r", "\n", "\u2028", "\n", "\u2029", "\n").Replace(text)
	return parseCreditLines(strings.Split(text, "\n")), nil
}

// Headings after which a show page lists other shows ("Vous aimerez aussi"):
// credits found below them belong to those shows. Les Théâtres' page for
// "Fête des mères" offered another show's company as its own, pre-checked.
var otherShowsHeading = regexp.MustCompile(`^(vous aimerez (aussi|également)|vous aimerez peut-être|à (voir|découvrir) aussi|dans la même (thématique|catégorie)|les autres spectacles|autres spectacles|aussi à l'affiche|aussi à l’affiche|prochainement|spectacles associés|en lien avec ce spectacle)\b`)

// ownShowLines keeps a page's lines up to the first "other shows" heading.
// Only for pages: pasted text is the user's own selection.
func ownShowLines(lines []string) []string {
	for i, l := range lines {
		if otherShowsHeading.MatchString(strings.ToLower(strings.TrimSpace(l))) {
			return lines[:i]
		}
	}
	return lines
}
