package live

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sbondCo/Watcharr/domain"
)

// pageServer serves fixed pages by path; /moved redirects to /full.
func pageServer(t *testing.T, pages map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/moved":
			http.Redirect(w, r, "/full", http.StatusMovedPermanently)
			return
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"title":"x"}`))
			return
		}
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const fullPage = `<!DOCTYPE html>
<html><head>
<title>Ignored &ndash; when og:title exists</title>
<meta property="og:title" content="  Le   Misanthrope
  de Moli&egrave;re ">
<meta property="og:description" content="Alceste &amp; Célimène.">
<meta name="description" content="not this one">
<meta property="og:site_name" content="La Criée">
<meta property="og:image" content="/media/affiche.jpg?w=800&amp;h=1200">
<meta property="og:title" content="second og:title loses">
</head>
<body>
<meta property="og:site_name" content="in the body, too late">
</body></html>`

const fallbackPage = `<html><head>
<TITLE> Fallback
 title </TITLE>
<meta name="Description" content="From the meta description.">
<meta name="twitter:image" content="https://cdn.example.org/poster.png">
<svg><title>not the page title</title></svg>
</head><body></body></html>`

// No </head>: the body start must end parsing.
const bodyStopPage = `<html><head><title>T</title>
<body><meta property="og:description" content="from the body">`

func TestPrefillEndToEnd(t *testing.T) {
	s := newTestService(t)
	srv := pageServer(t, map[string]string{
		"/full":     fullPage,
		"/fallback": fallbackPage,
		"/stop":     bodyStopPage,
		"/empty":    "<html><head></head><body>nothing</body></html>",
	})

	got, err := s.Prefill(context.Background(), 1, srv.URL+"/moved")
	if err != nil {
		t.Fatalf("Prefill: %v", err)
	}
	want := domain.LivePrefill{
		URL:         srv.URL + "/full",
		Title:       "Le Misanthrope de Molière",
		Description: "Alceste & Célimène.",
		Venue:       "La Criée",
		PosterURL:   srv.URL + "/media/affiche.jpg?w=800&h=1200",
		// Never nil, so the JSON is [] and not null.
		Performances: []string{},
		Credits:      []domain.LiveCreditSuggestion{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("full page:\n got %+v\nwant %+v", got, want)
	}

	got, err = s.Prefill(context.Background(), 1, srv.URL+"/fallback")
	if err != nil {
		t.Fatalf("Prefill: %v", err)
	}
	want = domain.LivePrefill{
		URL:          srv.URL + "/fallback",
		Title:        "Fallback title",
		Description:  "From the meta description.",
		PosterURL:    "https://cdn.example.org/poster.png",
		Performances: []string{},
		Credits:      []domain.LiveCreditSuggestion{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fallback page:\n got %+v\nwant %+v", got, want)
	}

	got, err = s.Prefill(context.Background(), 1, srv.URL+"/stop")
	if err != nil {
		t.Fatalf("Prefill: %v", err)
	}
	if got.Title != "T" || got.Description != "" {
		t.Errorf("parsing did not stop at <body>: %+v", got)
	}

	got, err = s.Prefill(context.Background(), 1, srv.URL+"/empty")
	if err != nil {
		t.Fatalf("Prefill: %v", err)
	}
	if !reflect.DeepEqual(got, domain.LivePrefill{URL: srv.URL + "/empty", Performances: []string{}, Credits: []domain.LiveCreditSuggestion{}}) {
		t.Errorf("empty page: %+v", got)
	}
}

func TestPrefillErrors(t *testing.T) {
	s := newTestService(t)
	srv := pageServer(t, nil)

	for _, raw := range []string{"", "not a url", "ftp://example.com/", "javascript:alert(1)"} {
		_, err := s.Prefill(context.Background(), 1, raw)
		var verr *ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("%q: want a *ValidationError, got %v", raw, err)
		}
	}
	for _, path := range []string{"/missing", "/json"} {
		if _, err := s.Prefill(context.Background(), 1, srv.URL+path); !errors.Is(err, errUnreadable) {
			t.Errorf("%s: want errUnreadable, got %v", path, err)
		}
	}
	// Nothing listening.
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	if _, err := s.Prefill(context.Background(), 1, dead.URL); !errors.Is(err, errUnreadable) {
		t.Errorf("closed server: want errUnreadable, got %v", err)
	}
}

// The production fetcher, against a local server: refused as a 400, before
// anything is fetched.
func TestPrefillBlocksLocalAddresses(t *testing.T) {
	s := newTestService(t)
	s.fetch = buildFetcher(false, true)
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()

	_, err := s.Prefill(context.Background(), 1, srv.URL)
	var verr *ValidationError
	if !errors.As(err, &verr) || !strings.Contains(verr.Msg, "not allowed") {
		t.Errorf("want a 'not allowed' ValidationError, got %v", err)
	}
	if hit {
		t.Error("the local server was reached")
	}
}

func TestBuildPrefillTruncatesCharacters(t *testing.T) {
	base, _ := url.Parse("https://example.com/show")
	m := pageMeta{props: map[string]string{
		"og:title":       strings.Repeat("é", maxTitle+10),
		"og:site_name":   strings.Repeat("à", maxVenue+10),
		"og:description": strings.Repeat("ç", maxDescription+10),
	}}
	p := buildPrefill(base, m)
	if n := utf8.RuneCountInString(p.Title); n != maxTitle {
		t.Errorf("title: %d characters", n)
	}
	if n := utf8.RuneCountInString(p.Venue); n != maxVenue {
		t.Errorf("venue: %d characters", n)
	}
	if n := utf8.RuneCountInString(p.Description); n != maxDescription {
		t.Errorf("description: %d characters", n)
	}
	// And the result passes validation as-is.
	if _, err := validate(domain.LiveShowRequest{
		Title: p.Title, Venue: p.Venue, Description: p.Description,
		URL: p.URL, Date: "2026-11-05T20:00:00Z",
	}); err != nil {
		t.Errorf("prefill result does not validate: %v", err)
	}
}

func TestResolveImage(t *testing.T) {
	base, _ := url.Parse("https://theatre.example/saison/show?x=1")
	for _, tc := range []struct {
		in   []string
		want string
	}{
		{[]string{"/img/a.jpg"}, "https://theatre.example/img/a.jpg"},
		{[]string{"a.jpg"}, "https://theatre.example/saison/a.jpg"},
		{[]string{"//cdn.example/a.jpg"}, "https://cdn.example/a.jpg"},
		{[]string{"http://other.example/a.jpg"}, "http://other.example/a.jpg"},
		{[]string{"", "  ", "/second.jpg"}, "https://theatre.example/second.jpg"},
		{[]string{"data:image/png;base64,AAAA", "/ok.png"}, "https://theatre.example/ok.png"},
		{[]string{"javascript:alert(1)"}, ""},
		{nil, ""},
	} {
		if got := resolveImage(base, tc.in...); got != tc.want {
			t.Errorf("resolveImage(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPrefillHTTPContract(t *testing.T) {
	e := testEngine(t)
	srv := pageServer(t, map[string]string{
		"/full": fullPage,
		"/bare": "<html><head></head></html>",
		"/dated": `<html><head></head><body><script type="application/ld+json">
{"@type":"TheaterEvent","name":"X","startDate":"2026-11-05T20:00"}</script></body></html>`,
		"/days": `<script type="application/ld+json">
{"@type":"Event","name":"X","startDate":"2027-04-07","endDate":"2027-04-09"}</script>`,
	})

	w := do(e, "POST", "/api/live/prefill", `{"url":"`+srv.URL+`/bare"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("prefill: %d %s", w.Code, w.Body)
	}
	var obj map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &obj); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k, v := range obj {
		keys = append(keys, k)
		switch k {
		case "performances", "credits":
			if a, ok := v.([]any); !ok || len(a) != 0 {
				t.Errorf("%s = %#v, want []", k, v)
			}
		case "dateRange":
			if v != nil {
				t.Errorf("dateRange = %#v, want null", v)
			}
		default:
			// Never null: the client fills the form with them directly.
			if _, ok := v.(string); !ok {
				t.Errorf("%s = %#v, want a string", k, v)
			}
		}
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "credits,dateRange,description,performances,posterUrl,title,url,venue" {
		t.Errorf("keys = %v", keys)
	}

	// The new fields filled in.
	w = do(e, "POST", "/api/live/prefill", `{"url":"`+srv.URL+`/dated"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("prefill: %d %s", w.Code, w.Body)
	}
	var dated struct {
		Performances []string `json:"performances"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &dated); err != nil {
		t.Fatal(err)
	}
	if len(dated.Performances) != 1 || dated.Performances[0] != "2026-11-05T20:00:00+01:00" {
		t.Errorf("performances = %v", dated.Performances)
	}
	w = do(e, "POST", "/api/live/prefill", `{"url":"`+srv.URL+`/days"}`)
	if !strings.Contains(w.Body.String(), `"dateRange":{"start":"2027-04-07","end":"2027-04-09"}`) {
		t.Errorf("dateRange: %s", w.Body)
	}

	for _, tc := range []struct {
		body string
		code int
		msg  string
	}{
		{`{"url":"` + srv.URL + `/full"}`, 200, ""},
		{`{"url":"ftp://example.com/"}`, 400, "url"},
		{`{"url":""}`, 400, "url"},
		{`not json`, 400, "invalid request body"},
		{`{"url":"` + srv.URL + `/missing"}`, 502, "could not read that page"},
	} {
		w := do(e, "POST", "/api/live/prefill", tc.body)
		if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.msg) {
			t.Errorf("%s: got %d %s, want %d %q", tc.body, w.Code, w.Body, tc.code, tc.msg)
		}
	}
}
