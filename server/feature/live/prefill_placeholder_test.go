package live

import (
	"net/url"
	"strings"
	"testing"
)

func TestPrefillIgnoresNullPlaceholders(t *testing.T) {
	page, _ := url.Parse("https://theatre.example/spectacle/boat-people")
	// Shape of a real La Criée page: description and twitter image are the
	// literal text "null", og:image is real.
	head := `<html><head>
<meta name="description" content="null">
<meta property="og:title" content="Boat People">
<meta property="og:site_name" content="undefined">
<meta name="twitter:image" content="null">
<meta property="og:image" content="NULL">
</head><body></body></html>`
	got := buildPrefill(page, parseHead(strings.NewReader(head)))
	if got.Title != "Boat People" {
		t.Errorf("title: got %q", got.Title)
	}
	if got.Description != "" {
		t.Errorf("a \"null\" description must be dropped, got %q", got.Description)
	}
	if got.Venue != "" {
		t.Errorf("an \"undefined\" site name must be dropped, got %q", got.Venue)
	}
	if got.PosterURL != "" {
		t.Errorf("\"null\" images must not resolve to a URL, got %q", got.PosterURL)
	}

	// A real word that merely contains "null" is kept.
	head = `<head><meta name="description" content="Nullement fait pour plaire"></head>`
	if got := buildPrefill(page, parseHead(strings.NewReader(head))); got.Description != "Nullement fait pour plaire" {
		t.Errorf("legit text was dropped: %q", got.Description)
	}
}
