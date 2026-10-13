package live

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
)

// seedShow creates a show for userId and backdates its updated_at, so tests
// control which one is "the most recent".
func seedShow(t *testing.T, s *Service, userId uint, link, venue string, updated time.Time) domain.LiveShow {
	t.Helper()
	show, err := s.Create(userId, domain.LiveShowRequest{
		Title: "Show", Venue: venue, URL: link, Date: "2026-11-05T20:00:00+01:00",
	})
	if err != nil {
		t.Fatalf("Create(%q): %v", link, err)
	}
	if err := s.db.Model(&entity.LiveShow{}).Where("id = ?", show.ID).
		UpdateColumn("updated_at", updated).Error; err != nil {
		t.Fatal(err)
	}
	return show
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestSiteHost(t *testing.T) {
	for raw, want := range map[string]string{
		"https://www.Theatre-LaCriee.com/x":  "theatre-lacriee.com",
		"https://WWW.theatre-lacriee.com/":   "theatre-lacriee.com",
		"http://theatre-lacriee.com:8080/y":  "theatre-lacriee.com",
		"https://billetterie.lacriee.com/z":  "billetterie.lacriee.com",
		"https://wwwtheatre.example/":        "wwwtheatre.example",
		"not a url":                          "",
		"":                                   "",
		"https://user:pw@www.example.org/?q": "example.org",
	} {
		if got := siteHost(raw); got != want {
			t.Errorf("siteHost(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestRememberedVenue(t *testing.T) {
	s := newTestService(t)
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := old.Add(24 * time.Hour)

	seedShow(t, s, 1, "https://www.theatre-lacriee.com/a", "Old name", old)
	seedShow(t, s, 1, "https://theatre-lacriee.com/b", "La Criée", recent)
	// Same site, no venue typed: never a candidate.
	seedShow(t, s, 1, "https://theatre-lacriee.com/c", "", recent.Add(time.Hour))
	// Newest of all, but deleted.
	gone := seedShow(t, s, 1, "https://theatre-lacriee.com/d", "Deleted venue", recent.Add(2*time.Hour))
	if err := s.Delete(1, gone.ID); err != nil {
		t.Fatal(err)
	}
	// Another user's show on the same site, newer: never used.
	seedShow(t, s, 2, "https://theatre-lacriee.com/e", "Someone else's", recent.Add(3*time.Hour))
	// Another site.
	seedShow(t, s, 1, "https://lestheatres.net/f", "Grand Théâtre", recent.Add(4*time.Hour))

	for _, tc := range []struct {
		user uint
		link string
		want string
	}{
		{1, "https://theatre-lacriee.com/spectacle/x", "La Criée"},
		{1, "https://WWW.Theatre-LaCriee.COM/spectacle/x", "La Criée"},
		{1, "https://lestheatres.net/g", "Grand Théâtre"},
		{1, "https://unknown.example/", ""},
		{2, "https://theatre-lacriee.com/x", "Someone else's"},
		{3, "https://theatre-lacriee.com/x", ""},
	} {
		if got := s.rememberedVenue(tc.user, mustURL(t, tc.link)); got != tc.want {
			t.Errorf("user %d, %s: got %q, want %q", tc.user, tc.link, got, tc.want)
		}
	}
}

func TestRememberedVenueMostRecentWins(t *testing.T) {
	s := newTestService(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Created newest first, so creation order is not what decides.
	seedShow(t, s, 1, "https://theatre.example/a", "Newer", base.Add(time.Hour))
	seedShow(t, s, 1, "https://theatre.example/b", "Older", base)
	if got := s.rememberedVenue(1, mustURL(t, "https://theatre.example/z")); got != "Newer" {
		t.Errorf("got %q, want Newer", got)
	}
}

// Priority through the real prefill: JSON-LD location, then memory, then
// og:site_name. The test server is 127.0.0.1, so seeded links use that host.
func TestPrefillVenuePriority(t *testing.T) {
	s := newTestService(t)
	srv := pageServer(t, map[string]string{
		"/none": `<html><head><title>T</title></head><body></body></html>`,
		"/og":   `<html><head><meta property="og:site_name" content="Site name"></head></html>`,
		"/ld": `<html><head><meta property="og:site_name" content="Site name"></head><body>
<script type="application/ld+json">{"@type":"Event","name":"A","location":{"@type":"Place","name":"Salle du page"}}</script>
</body></html>`,
	})
	host := mustURL(t, srv.URL).Host

	prefill := func(user uint, path string) string {
		t.Helper()
		p, err := s.Prefill(context.Background(), user, srv.URL+path)
		if err != nil {
			t.Fatalf("Prefill(%s): %v", path, err)
		}
		return p.Venue
	}

	// Nothing remembered yet.
	if got := prefill(1, "/none"); got != "" {
		t.Errorf("no memory, no venue on page: got %q", got)
	}
	if got := prefill(1, "/og"); got != "Site name" {
		t.Errorf("no memory: got %q, want og:site_name", got)
	}

	seedShow(t, s, 1, "http://"+host+"/earlier", "La Criée", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	if got := prefill(1, "/none"); got != "La Criée" {
		t.Errorf("page has no venue: got %q, want the remembered one", got)
	}
	if got := prefill(1, "/og"); got != "La Criée" {
		t.Errorf("memory must beat og:site_name: got %q", got)
	}
	if got := prefill(1, "/ld"); got != "Salle du page" {
		t.Errorf("JSON-LD location must beat memory: got %q", got)
	}
	if got := prefill(2, "/none"); got != "" {
		t.Errorf("another user's show was used: got %q", got)
	}
}
