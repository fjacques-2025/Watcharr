package live

import "strings"

// visibleText collects a page's visible text as lines, the way a reader sees
// it: credits are printed one per line, and a line break is as much a
// separator as a colon.
type visibleText struct {
	lines []string
	cur   strings.Builder
	// Depth inside elements whose text is not shown.
	hidden int
}

// Enough for any real show page.
const maxPageLines = 5000

var hiddenTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true,
	"svg": true, "title": true, "textarea": true,
}

// Elements that start a new line. Not table cells: a credits table puts the
// label and the names in neighbouring cells, which reads as "label names"
// (cellBreak spaces them apart).
var blockTags = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true,
	"body": true, "br": true, "dd": true, "div": true, "dl": true, "dt": true,
	"fieldset": true, "figcaption": true, "figure": true, "footer": true,
	"form": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true,
	"h6": true, "header": true, "hr": true, "li": true, "main": true,
	"nav": true, "ol": true, "p": true, "pre": true, "section": true,
	"table": true, "tr": true, "ul": true, "button": true, "option": true,
	"select": true, "summary": true, "details": true,
}

// Cells of a row stay on one line, but must not glue: "Texte" "Jean".
var cellBreak = map[string]bool{"td": true, "th": true}

func (v *visibleText) tag(name string, opens bool) {
	if cellBreak[name] {
		v.cur.WriteByte(' ')
	}
	if hiddenTags[name] {
		// A self-closing <svg/> holds nothing to hide.
		if opens {
			v.flush()
			v.hidden++
		}
		return
	}
	if blockTags[name] {
		v.flush()
	}
}

func (v *visibleText) endTag(name string) {
	if cellBreak[name] {
		v.cur.WriteByte(' ')
	}
	if hiddenTags[name] {
		if v.hidden > 0 {
			v.hidden--
		}
		return
	}
	if blockTags[name] {
		v.flush()
	}
}

func (v *visibleText) text(t []byte) {
	if v.hidden > 0 || len(v.lines) >= maxPageLines {
		return
	}
	// Inline elements glue as rendered; source newlines inside the text are
	// spaces on screen, and flush collapses them.
	v.cur.Write(t)
}

func (v *visibleText) flush() {
	// Fields also splits on NBSP and narrow NBSP (unicode White_Space).
	if l := collapseSpaces(v.cur.String()); l != "" && len(v.lines) < maxPageLines {
		v.lines = append(v.lines, l)
	}
	v.cur.Reset()
}

func (v *visibleText) done() []string {
	v.flush()
	return v.lines
}
