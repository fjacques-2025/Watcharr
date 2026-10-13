package live

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"strings"

	"github.com/sbondCo/Watcharr/domain"
	"golang.org/x/net/html"
)

// errUnreadable is any failure to get an HTML page out of a valid, allowed
// link: a 502, the page is the problem, not the request.
var errUnreadable = errors.New("could not read that page")

// Prefill reads the page behind a link and proposes the fields it can fill.
// Everything comes back within the validation limits, so the form can be
// saved as-is.
func (s *Service) Prefill(ctx context.Context, rawURL string) (domain.LivePrefill, error) {
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
	m := parseHead(bytes.NewReader(page.Body))
	return buildPrefill(page.FinalURL, m), nil
}

// headMeta is what we pick from <head>, raw (still to be cleaned).
type headMeta struct {
	title    string // <title>
	props    map[string]string
	metaDesc string // <meta name="description">
}

// parseHead tokenizes up to </head> or <body>: the metadata is all there and
// the rest of the page can be huge.
func parseHead(r io.Reader) headMeta {
	m := headMeta{props: map[string]string{}}
	z := html.NewTokenizer(r)
	inTitle := false
	for {
		switch z.Next() {
		case html.ErrorToken:
			return m
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch string(name) {
			case "body":
				return m
			case "title":
				inTitle = true
			case "meta":
				if hasAttr {
					m.readMeta(z)
				}
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "head":
				return m
			case "title":
				inTitle = false
			}
		case html.TextToken:
			// First <title> wins; an SVG <title> further down is not ours.
			if inTitle && m.title == "" {
				m.title = string(z.Text())
			}
		}
	}
}

func (m *headMeta) readMeta(z *html.Tokenizer) {
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

func buildPrefill(pageURL *url.URL, m headMeta) domain.LivePrefill {
	first := func(vals ...string) string {
		for _, v := range vals {
			if v = cleanText(v); v != "" {
				return v
			}
		}
		return ""
	}
	return domain.LivePrefill{
		URL:         pageURL.String(),
		Title:       truncateRunes(first(m.props["og:title"], m.title), maxTitle),
		Description: truncateRunes(first(m.props["og:description"], m.metaDesc), maxDescription),
		Venue:       truncateRunes(first(m.props["og:site_name"]), maxVenue),
		PosterURL: resolveImage(pageURL,
			m.props["og:image"], m.props["og:image:url"], m.props["twitter:image"]),
	}
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
