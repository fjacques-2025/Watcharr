package live

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/sbondCo/Watcharr/domain"
	"golang.org/x/net/html"
)

// errUnreadable is any failure to get an HTML page out of a valid, allowed
// link: a 502, the page is the problem, not the request.
var errUnreadable = errors.New("could not read that page")

// Prefill reads the page behind a link and proposes the fields it can fill.
// Everything comes back within the validation limits, so the form can be
// saved as-is. userId is whose shows the venue may be remembered from.
func (s *Service) Prefill(ctx context.Context, userId uint, rawURL string) (domain.LivePrefill, error) {
	u, err := s.fetch.parseURL(rawURL)
	if err != nil {
		return domain.LivePrefill{}, err
	}
	page, err := s.fetch.get(ctx, u, maxPageBytes, true)
	if err != nil {
		if errors.Is(err, errBlockedAddress) {
			slog.Warn("live: prefill refused", "url", u.Redacted(), "reason", err)
			return domain.LivePrefill{}, invalid("that address is not allowed")
		}
		var verr *ValidationError
		if errors.As(err, &verr) {
			// A redirect to a scheme or port we do not follow.
			slog.Warn("live: prefill refused", "url", u.Redacted(), "reason", err)
			return domain.LivePrefill{}, verr
		}
		slog.Warn("live: prefill fetch failed", "url", u.Redacted(), "reason", err)
		return domain.LivePrefill{}, errUnreadable
	}
	if !isHTML(page.ContentType, page.Body) {
		slog.Warn("live: prefill not HTML", "url", page.FinalURL.Redacted(), "content_type", page.ContentType)
		return domain.LivePrefill{}, errUnreadable
	}
	m := parsePage(bytes.NewReader(page.Body))
	return buildPrefillWithVenue(page.FinalURL, m, s.rememberedVenue(userId, page.FinalURL)), nil
}

// pageMeta is what we pick from the page, raw (still to be cleaned).
type pageMeta struct {
	title    string // <title>
	props    map[string]string
	metaDesc string // <meta name="description">
	events   []ldEvent
	images   []string // <img> sources in document order, for posterFromPage
	lines    []string // visible text, one entry per block, for the credits
}

// Enough for any real show page; a gallery page needn't be read to its end.
const maxPageImages = 300

// parsePage reads <title> and <meta> from the head only (a <meta> in the body
// is not the page's), and JSON-LD from anywhere: some sites print it at the
// end of the body. The fetcher already capped the page size.
func parsePage(r io.Reader) pageMeta {
	m := pageMeta{props: map[string]string{}}
	z := html.NewTokenizer(r)
	inHead, inTitle, inLD := true, false, false
	var text visibleText
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			m.lines = text.done()
			return m
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			text.tag(string(name), tt == html.StartTagToken)
			switch string(name) {
			case "body":
				inHead = false
			case "title":
				inTitle = inHead
			case "meta":
				if inHead && hasAttr {
					m.readMeta(z)
				}
			case "script":
				inLD = hasAttr && isLDScript(z)
			case "img":
				if hasAttr && len(m.images) < maxPageImages {
					m.images = append(m.images, imgSources(z)...)
				}
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			text.endTag(string(name))
			switch string(name) {
			case "head":
				inHead = false
			case "title":
				inTitle = false
			case "script":
				inLD = false
			}
		case html.TextToken:
			// Text() empties the token: read it once.
			t := z.Text()
			// First <title> wins; an SVG <title> further down is not ours.
			if inTitle && m.title == "" {
				m.title = string(t)
			}
			if inLD {
				parseJSONLD(t, &m.events)
			}
			text.text(t)
		}
	}
}

// imgSources returns an <img>'s source, lazy-loading attributes first: with
// lazy loading, src is often a placeholder.
func imgSources(z *html.Tokenizer) []string {
	var lazy, src string
	for {
		k, v, more := z.TagAttr()
		switch strings.ToLower(string(k)) {
		case "data-src", "data-lazy-src", "data-original":
			if lazy == "" {
				lazy = string(v)
			}
		case "src":
			src = string(v)
		}
		if !more {
			break
		}
	}
	var out []string
	for _, c := range []string{lazy, src} {
		if c = strings.TrimSpace(c); c != "" && !strings.HasPrefix(c, "data:") {
			out = append(out, c)
		}
	}
	return out
}

func isLDScript(z *html.Tokenizer) bool {
	for {
		k, v, more := z.TagAttr()
		if strings.EqualFold(string(k), "type") {
			return strings.EqualFold(strings.TrimSpace(string(v)), "application/ld+json")
		}
		if !more {
			return false
		}
	}
}

func (m *pageMeta) readMeta(z *html.Tokenizer) {
	var key, name, content string
	hasContent := false
	for {
		k, v, more := z.TagAttr()
		switch strings.ToLower(string(k)) {
		// Open Graph uses `property`, but plenty of sites put it in `name`.
		case "property":
			key = strings.ToLower(strings.TrimSpace(string(v)))
		case "name":
			name = strings.ToLower(strings.TrimSpace(string(v)))
		case "content":
			content = string(v)
			hasContent = true
		}
		if !more {
			break
		}
	}
	if !hasContent {
		return
	}
	if key == "" {
		key = name
	}
	if name == "description" && m.metaDesc == "" {
		m.metaDesc = content
	}
	if key != "" {
		// First one wins, like crawlers do.
		if _, ok := m.props[key]; !ok {
			m.props[key] = content
		}
	}
}

func buildPrefill(pageURL *url.URL, m pageMeta) domain.LivePrefill {
	return buildPrefillWithVenue(pageURL, m, "")
}

// buildPrefillWithVenue is buildPrefill with the venue the user last typed
// for a show from the same site: it beats og:site_name (often the site's
// name rather than the hall's) but not a venue the page itself declares.
func buildPrefillWithVenue(pageURL *url.URL, m pageMeta, remembered string) domain.LivePrefill {
	first := func(vals ...string) string {
		for _, v := range vals {
			if v = cleanText(v); v != "" {
				return v
			}
		}
		return ""
	}
	// A page with one event per performance repeats the same show: the
	// first value given wins for each field.
	var ldName, ldDesc, ldVenue string
	var ldImages []string
	for _, e := range m.events {
		if ldName == "" {
			ldName = cleanText(e.name)
		}
		if ldDesc == "" {
			ldDesc = cleanText(e.description)
		}
		if ldVenue == "" {
			ldVenue = cleanText(e.location)
		}
		ldImages = append(ldImages, e.images...)
	}
	site := first(m.props["og:site_name"])

	// The event name is the bare title; og:title and <title> tend to append
	// credits and the site's name.
	title := stripSiteSuffix(first(ldName, m.props["og:title"], m.title), site)

	// Neither source is reliably the synopsis: Les Théâtres puts the credits
	// in JSON-LD and the synopsis in og, Joliette the full synopsis in JSON-LD
	// and a truncated one in og. The longer one is the better bet.
	desc := first(m.props["og:description"])
	if utf8.RuneCountInString(ldDesc) > utf8.RuneCountInString(desc) {
		desc = ldDesc
	}
	if desc == "" {
		desc = first(m.metaDesc)
	}

	perfs, rng := eventDates(m.events)
	return domain.LivePrefill{
		URL:         pageURL.String(),
		Title:       truncateRunes(title, maxTitle),
		Description: truncateRunes(desc, maxDescription),
		Venue:       truncateRunes(first(ldVenue, remembered, site), maxVenue),
		// JSON-LD images last: they are often originals, which can exceed
		// what the poster saver accepts.
		PosterURL:    poster(pageURL, title, m, ldImages),
		Performances: perfs,
		DateRange:    rng,
		Credits:      parseCreditLines(ownShowLines(m.lines)),
	}
}

// poster: the image the page declares, else one of its images named after
// the show. The Opéra-Odéon's pages carry og:image:alt but no og:image, while
// the poster in the page is "la-jalousie-….png" for /programmation/la-jalousie.
func poster(pageURL *url.URL, title string, m pageMeta, ldImages []string) string {
	// JSON-LD images last: they are often originals, which can exceed what
	// the poster saver accepts.
	declared := resolveImage(pageURL, append([]string{
		m.props["og:image"], m.props["og:image:url"], m.props["twitter:image"],
	}, ldImages...)...)
	if declared != "" {
		return declared
	}
	return posterFromPage(pageURL, title, m.images)
}

var leadingID = regexp.MustCompile(`^[0-9]+-`)

// posterFromPage picks the first page image whose file name contains the
// show's slug, taken from the page URL ("…/6102-fete-des-meres" → "fete-des-
// meres") or from the title. Nothing matches → nothing: a theatre's logo is
// worse than no poster. Formats the poster saver can't store are skipped.
func posterFromPage(pageURL *url.URL, title string, images []string) string {
	var keys []string
	for _, k := range []string{
		leadingID.ReplaceAllString(slugFold(path.Base(strings.TrimSuffix(pageURL.Path, "/"))), ""),
		slugFold(title),
	} {
		if len(k) >= 4 {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	for _, c := range images {
		abs := resolveImage(pageURL, c)
		if abs == "" {
			continue
		}
		u, err := url.Parse(abs)
		if err != nil {
			continue
		}
		base := path.Base(u.Path)
		ext := strings.ToLower(path.Ext(base))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" {
			continue
		}
		name := slugFold(strings.TrimSuffix(base, path.Ext(base)))
		if strings.Contains(name, "logo") || strings.Contains(name, "icon") {
			continue
		}
		for _, k := range keys {
			if strings.Contains(name, k) {
				return abs
			}
		}
	}
	return ""
}

// slugFold lowers case, drops accents and turns every run of other
// characters into one "-": "LA JALOUSIE" and "la-jalousie" agree.
func slugFold(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range searchFold(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

var titleSeparators = []string{" | ", " - ", " – ", " — ", " • ", " · "}

// stripSiteSuffix drops a trailing " | Site name…" segment, as in
// "Show - Credits | Les Théâtres • Saison 26•27". Only when we know the site
// name: guessing from separators alone would cut real titles. Case is kept,
// lowering it would mangle proper nouns.
func stripSiteSuffix(title, site string) string {
	if site == "" {
		return title
	}
	lowSite := strings.ToLower(site)
	// Twice, for "Show | Site | Site" style titles.
	for range 2 {
		cut := -1
		for _, sep := range titleSeparators {
			for i := strings.Index(title, sep); i >= 0; {
				rest := title[i+len(sep):]
				if i > 0 && strings.HasPrefix(strings.ToLower(rest), lowSite) && i > cut {
					cut = i
				}
				next := strings.Index(rest, sep)
				if next < 0 {
					break
				}
				i += len(sep) + next
			}
		}
		if cut < 0 {
			break
		}
		title = strings.TrimSpace(title[:cut])
	}
	return title
}

// cleanText unescapes entities (the tokenizer already did once; some sites
// double-escape), and collapses whitespace.
func cleanText(s string) string {
	s = strings.Join(strings.Fields(html.UnescapeString(s)), " ")
	if isPlaceholder(s) {
		return ""
	}
	return s
}

// isPlaceholder spots a JavaScript empty value printed as text. La Criée's
// pages carry <meta name="description" content="null">: taken literally it
// would become the show's description, or a poster URL ending in "/null".
func isPlaceholder(s string) bool {
	switch strings.ToLower(s) {
	case "null", "undefined", "nil", "none":
		return true
	}
	return false
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max]))
}

// resolveImage returns the first candidate that resolves to an absolute
// http(s) URL against the page. Not fetched here: that happens on save.
func resolveImage(base *url.URL, candidates ...string) string {
	for _, c := range candidates {
		c = strings.TrimSpace(html.UnescapeString(c))
		if c == "" || isPlaceholder(c) {
			continue
		}
		ref, err := url.Parse(c)
		if err != nil {
			continue
		}
		abs := base.ResolveReference(ref)
		if (abs.Scheme == "http" || abs.Scheme == "https") && abs.Host != "" {
			return abs.String()
		}
	}
	return ""
}
