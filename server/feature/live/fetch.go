package live

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// The server fetches URLs the user pastes, from the user's home NAS. Every
// outbound request of this feature goes through fetcher so that a link (or a
// redirect, or a DNS answer) cannot point it at the LAN, the router admin page
// or the NAS itself.

const (
	fetchTimeout = 10 * time.Second
	maxRedirects = 5
	// Only the <head> matters, so a cut-off page is fine.
	maxPageBytes = 2 << 20
	// Same ceiling as the image saver.
	maxImageBytes = 10 << 20
	userAgent     = "Watcharr (self-hosted; fetching a page the user linked to)"
)

// errBlockedAddress is a destination we refuse to reach: reported as a 400,
// the link itself is the problem.
var errBlockedAddress = errors.New("address not allowed")

// blockedPrefixes are the non-public ranges netip has no predicate for.
var blockedPrefixes = func() []netip.Prefix {
	var out []netip.Prefix
	for _, s := range []string{
		"0.0.0.0/8",     // "this network"
		"100.64.0.0/10", // CGNAT, often the ISP box side of the NAS
		"192.0.0.0/24",  // IETF protocol assignments
		"198.18.0.0/15", // benchmarking
		"240.0.0.0/4",   // reserved, broadcast included
		"::/96",         // IPv4-compatible (deprecated), would smuggle a v4
		"64:ff9b::/96",  // NAT64: reaches arbitrary v4, internal ones too
		"64:ff9b:1::/48",
		"2002::/16", // 6to4, embeds a v4
	} {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()

// isPublicAddr reports whether we may connect to ip. IPv4-mapped IPv6
// (::ffff:a.b.c.d) is judged as the v4 it carries.
func isPublicAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() ||
		ip.IsLoopback() ||
		ip.IsPrivate() || // RFC1918 and fc00::/7
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() {
		return false
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

type fetcher struct {
	client *http.Client
	// Test-only escape hatches: httptest servers live on 127.0.0.1 with a
	// random port. newFetcher never sets them.
	allowPrivate bool
	allowAnyPort bool
}

func newFetcher() *fetcher {
	return buildFetcher(false, false)
}

func buildFetcher(allowPrivate, allowAnyPort bool) *fetcher {
	f := &fetcher{allowPrivate: allowPrivate, allowAnyPort: allowAnyPort}
	dialer := &net.Dialer{
		Timeout: fetchTimeout,
		// Control runs on the address actually being dialled, after DNS: this
		// is what defeats DNS rebinding and redirects to internal hosts.
		Control: func(network, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil {
				return fmt.Errorf("%w: %s", errBlockedAddress, address)
			}
			if !f.allowPrivate && !isPublicAddr(ap.Addr()) {
				return fmt.Errorf("%w: %s", errBlockedAddress, ap.Addr())
			}
			if !f.allowAnyPort && ap.Port() != 80 && ap.Port() != 443 {
				return fmt.Errorf("%w: port %d", errBlockedAddress, ap.Port())
			}
			return nil
		},
	}
	transport := &http.Transport{
		// No proxy: through one, we would dial the proxy and the check above
		// would judge the wrong address.
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   fetchTimeout,
		ResponseHeaderTimeout: fetchTimeout,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
	}
	f.client = &http.Client{
		Transport: transport,
		Timeout:   fetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			// Each hop dials through the same Control; this only rejects the
			// schemes and ports up front.
			return f.checkURL(req.URL)
		},
	}
	return f
}

// parseURL checks a user-supplied link before any network access.
func (f *fetcher) parseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return nil, invalid("url must be an absolute http or https link")
	}
	if err := f.checkURL(u); err != nil {
		return nil, err
	}
	return u, nil
}

func (f *fetcher) checkURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return invalid("url must be an absolute http or https link")
	}
	if u.Hostname() == "" {
		return invalid("url must be an absolute http or https link")
	}
	if p := u.Port(); p != "" && p != "80" && p != "443" && !f.allowAnyPort {
		return invalid("url must use the standard port (80 or 443)")
	}
	return nil
}

// fetched is a 2xx response, body read up to the limit.
type fetched struct {
	FinalURL    *url.URL
	ContentType string
	Body        []byte
}

// errTooBig is an image over maxImageBytes.
var errTooBig = errors.New("file is too big")

// get fetches u. Past maxBytes, the body is cut off when truncate is set (a
// page) and is an error otherwise (an image, which must be whole).
func (f *fetcher) get(ctx context.Context, u *url.URL, maxBytes int64, truncate bool) (*fetched, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	b, err := io.ReadAll(&io.LimitedReader{R: res.Body, N: maxBytes + 1})
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxBytes {
		if !truncate {
			return nil, errTooBig
		}
		b = b[:maxBytes]
	}
	return &fetched{
		FinalURL:    res.Request.URL,
		ContentType: res.Header.Get("Content-Type"),
		Body:        b,
	}, nil
}

// isHTML trusts the declared type, and sniffs only when there is none.
func isHTML(contentType string, body []byte) bool {
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mt == "text/html" || mt == "application/xhtml+xml"
}
