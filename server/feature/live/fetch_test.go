package live

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

func TestIsPublicAddr(t *testing.T) {
	blocked := []string{
		// loopback
		"127.0.0.1", "127.255.255.254", "::1",
		// RFC1918
		"10.0.0.1", "10.255.255.255", "172.16.0.1", "172.31.255.255",
		"192.168.0.1", "192.168.1.254",
		// IPv6 ULA fc00::/7
		"fc00::1", "fd12:3456:789a::1",
		// link-local, cloud metadata included
		"169.254.0.1", "169.254.169.254", "fe80::1", "fe80::1%eth0",
		// multicast
		"224.0.0.1", "239.255.255.250", "ff02::1", "ff05::2", "ff01::1",
		// unspecified
		"0.0.0.0", "::",
		// CGNAT 100.64.0.0/10
		"100.64.0.1", "100.127.255.254",
		// misc non-public v4
		"0.1.2.3", "192.0.0.8", "198.18.0.1", "198.19.255.255",
		"240.0.0.1", "255.255.255.255",
		// IPv4-mapped forms
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:192.168.1.1",
		"::ffff:169.254.169.254", "::ffff:100.64.0.1", "::ffff:0.0.0.0",
		"::ffff:224.0.0.1",
		// v4 smuggled in other v6 forms
		"::127.0.0.1", "::10.0.0.1", "64:ff9b::7f00:1", "64:ff9b:1::a00:1",
		"2002:7f00:1::1",
	}
	for _, s := range blocked {
		if isPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	allowed := []string{
		"8.8.8.8", "1.1.1.1", "93.184.215.14",
		// Just outside the ranges.
		"100.63.255.255", "100.128.0.0", "172.15.255.255", "172.32.0.0",
		"169.253.255.255", "11.0.0.1", "198.17.255.255", "223.255.255.255",
		"::ffff:8.8.8.8",
		"2001:4860:4860::8888", "2a00:1450:4007:80e::200e",
	}
	for _, s := range allowed {
		if !isPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
	if isPublicAddr(netip.Addr{}) {
		t.Error("the zero Addr should be blocked")
	}
}

func TestProductionFetcherAlwaysBlocks(t *testing.T) {
	f := newFetcher()
	if f.allowPrivate || f.allowAnyPort {
		t.Fatal("newFetcher must not relax any check")
	}
	if f.client.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("a proxy would bypass the dial-time check")
	}
}

// The check happens at dial time, on the resolved address: a loopback server
// is refused whatever the URL looks like (literal IP or a name resolving to
// it, which is what DNS rebinding relies on).
func TestDialTimeBlocksLoopback(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
	}))
	defer srv.Close()
	// Ports relaxed so that only the address check is in play.
	f := buildFetcher(false, true)

	su, _ := url.Parse(srv.URL)
	for _, raw := range []string{
		srv.URL,
		"http://localhost:" + su.Port(),
	} {
		u, err := f.parseURL(raw)
		if err != nil {
			t.Fatalf("parseURL(%s): %v", raw, err)
		}
		_, err = f.get(context.Background(), u, maxPageBytes, true)
		if !errors.Is(err, errBlockedAddress) {
			t.Errorf("%s: want errBlockedAddress, got %v", raw, err)
		}
	}
	if hit {
		t.Error("the loopback server was reached")
	}
}

func TestDialTimeBlocksOtherPorts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	// Addresses relaxed, ports not: the URL check is skipped by calling get
	// directly, so the dialer has to catch it.
	f := buildFetcher(true, false)
	u, _ := url.Parse(srv.URL)
	if _, err := f.get(context.Background(), u, maxPageBytes, true); !errors.Is(err, errBlockedAddress) {
		t.Errorf("want errBlockedAddress, got %v", err)
	}
}

func TestParseURL(t *testing.T) {
	f := newFetcher()
	for _, raw := range []string{
		"https://example.com/x", "http://example.com", "https://example.com:443/x",
		"http://example.com:80/", " https://example.com/x ",
	} {
		if _, err := f.parseURL(raw); err != nil {
			t.Errorf("%q: unexpected %v", raw, err)
		}
	}
	for _, raw := range []string{
		"", "example.com", "/x", "ftp://example.com/", "file:///etc/passwd",
		"javascript:alert(1)", "https://", "http://example.com:8080/",
		"https://example.com:22/", "gopher://example.com:70/",
	} {
		_, err := f.parseURL(raw)
		var verr *ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("%q: want a *ValidationError, got %v", raw, err)
		}
	}
}

func TestCheckRedirect(t *testing.T) {
	f := newFetcher()
	req := func(raw string) *http.Request {
		r, _ := http.NewRequest("GET", raw, nil)
		return r
	}
	if err := f.client.CheckRedirect(req("https://example.com/b"), []*http.Request{req("https://example.com/a")}); err != nil {
		t.Errorf("plain redirect refused: %v", err)
	}
	for _, raw := range []string{"ftp://example.com/", "http://example.com:8080/", "file:///etc/passwd"} {
		if err := f.client.CheckRedirect(req(raw), []*http.Request{req("https://example.com/a")}); err == nil {
			t.Errorf("redirect to %s accepted", raw)
		}
	}
	via := make([]*http.Request, maxRedirects)
	for i := range via {
		via[i] = req("https://example.com/a")
	}
	if err := f.client.CheckRedirect(req("https://example.com/b"), via); err == nil {
		t.Error("redirect past the limit accepted")
	}
}

func TestRedirectLimitEndToEnd(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/again", http.StatusFound)
	}))
	defer srv.Close()
	f := buildFetcher(true, true)
	u, _ := url.Parse(srv.URL)
	_, err := f.get(context.Background(), u, maxPageBytes, true)
	if err == nil || !strings.Contains(err.Error(), "too many redirects") {
		t.Errorf("want too many redirects, got %v", err)
	}
}

func TestGetLimitsAndUserAgent(t *testing.T) {
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.UserAgent()
		n := maxPageBytes + 1000
		if r.URL.Path == "/img" {
			n = maxImageBytes + 1
		}
		w.Write([]byte(strings.Repeat("a", n)))
	}))
	defer srv.Close()
	f := buildFetcher(true, true)

	u, _ := url.Parse(srv.URL + "/page")
	page, err := f.get(context.Background(), u, maxPageBytes, true)
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if len(page.Body) != maxPageBytes {
		t.Errorf("page not truncated to the limit: %d bytes", len(page.Body))
	}
	if ua != userAgent {
		t.Errorf("User-Agent = %q", ua)
	}

	u, _ = url.Parse(srv.URL + "/img")
	if _, err := f.get(context.Background(), u, maxImageBytes, false); !errors.Is(err, errTooBig) {
		t.Errorf("image: want errTooBig, got %v", err)
	}
}

func TestIsHTML(t *testing.T) {
	for _, tc := range []struct {
		ct   string
		body string
		want bool
	}{
		{"text/html", "", true},
		{"text/html; charset=utf-8", "", true},
		{"TEXT/HTML", "", true},
		{"application/xhtml+xml", "", true},
		{"application/json", "<html>", false},
		{"image/png", "", false},
		{"", "<!DOCTYPE html><html><head>", true},
		{"", "\x89PNG\r\n\x1a\n", false},
	} {
		if got := isHTML(tc.ct, []byte(tc.body)); got != tc.want {
			t.Errorf("isHTML(%q, %q) = %v", tc.ct, tc.body, got)
		}
	}
}
