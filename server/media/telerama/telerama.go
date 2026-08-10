// Package telerama reads Télérama's film ratings.
//
// Télérama rates films from 0 to 4 "T"s and puts both the rating and the review
// behind a subscriber paywall on the article page. We do not touch that: this
// package reads the *public* rating filter of their film index instead.
//
//	https://www.telerama.fr/liste/films/4t/   -> the films rated "Bravo"
//	                            …/3t/, /2t/, /1t/, /0t/
//
// Each film appears in exactly one bucket, so fetching the five listings gives
// the rating without reading a single paywalled page. The review text stays
// where it belongs — behind the link, which opens the Télérama app on mobile
// for a subscriber.
//
// This is a scraper: it breaks when the template changes, and TestParseList is
// what tells you. Callers must cache; the index only moves on release day.
package telerama

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	gocache "github.com/robfig/go-cache"
	"github.com/sbondCo/Watcharr/cache"
	"golang.org/x/net/html"
)

// Review is Télérama's verdict on one film.
type Review struct {
	// Title as Télérama lists it.
	Title string `json:"title"`
	// Director, as listed. Useful to disambiguate reissues sharing a title.
	Director string `json:"director,omitempty"`
	// Production year, 0 when not listed.
	Year int `json:"year,omitempty"`
	// Number of "T"s, 0 to 4.
	Rating int `json:"rating"`
	// Télérama's own word for that rating ("Bravo", "Bien", …). Their scale is
	// editorial, so their wording carries it better than a number out of 4.
	RatingLabel string `json:"ratingLabel"`
	// Absolute URL of the review. On mobile this opens the Télérama app.
	URL string `json:"url"`
}

const baseURL = "https://www.telerama.fr"

// Identify ourselves rather than pretending to be a browser.
const userAgent = "Watcharr (self-hosted watch list; ratings for the user's own cinema listings)"

// buckets maps the public filter path to the rating it holds, with Télérama's
// own label for it.
var buckets = []struct {
	Path   string
	Rating int
	Label  string
}{
	{"/liste/films/4t/", 4, "Bravo"},
	{"/liste/films/3t/", 3, "Très Bien"},
	{"/liste/films/2t/", 2, "Bien"},
	{"/liste/films/1t/", 1, "Bof"},
	{"/liste/films/0t/", 0, "Hélas"},
}

// The index only changes when new reviews are published, so a long TTL is both
// safe and the polite thing to do — this is five page loads per refresh.
const cacheTTL = 12 * time.Hour

var (
	indexStore = gocache.New(cacheTTL, time.Hour)
	httpClient = &http.Client{Timeout: 20 * time.Second}
	yearRe     = regexp.MustCompile(`\b(19|20)\d{2}\b`)
)

// FetchIndex returns every film Télérama has recently reviewed, with its rating.
//
// A bucket that fails is logged and skipped: a partial index costs us some
// ratings, whereas failing outright would cost the caller all of them.
func FetchIndex() ([]Review, error) {
	cacheKey := cache.CreateCacheKey("TeleramaIndex")
	cached := new([]Review)
	if cache.GetCache(indexStore, cacheKey, &cached) {
		slog.Debug("telerama: FetchIndex returning cache.")
		return *cached, nil
	}

	var all []Review
	for _, b := range buckets {
		reviews, err := fetchBucket(b.Path, b.Rating, b.Label)
		if err != nil {
			slog.Error("telerama: skipping rating bucket",
				"path", b.Path, "error", err)
			continue
		}
		all = append(all, reviews...)
	}
	if len(all) == 0 {
		return nil, errors.New("telerama index came back empty")
	}
	slog.Debug("telerama: FetchIndex built", "reviews", len(all))
	indexStore.Set(cacheKey, &all, cacheTTL)
	return all, nil
}

func fetchBucket(path string, rating int, label string) ([]Review, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request returned %d", resp.StatusCode)
	}
	reviews, err := ParseList(resp.Body)
	if err != nil {
		return nil, err
	}
	for i := range reviews {
		reviews[i].Rating = rating
		reviews[i].RatingLabel = label
	}
	return reviews, nil
}

// ParseList reads one rating bucket's listing.
//
// Shape it relies on (see the testdata fixture):
//
//	<h2 class="search__card-subtitle search__card-subtitle--item">
//	  <a href="/cinema/…_cri-7045338.php"
//	     class="search__card-content-title-link">Girl</a>
//	</h2>
//	<span class="search__card-author">Qi Shu</span>              director
//	<span class="search__card-author">Drame … 2026</span>         genre + year
//
// Rating is not read here — it is implied by which bucket was fetched.
func ParseList(r io.Reader) ([]Review, error) {
	var (
		out []Review
		// Text sink, until captureTag closes.
		sink       *string
		captureTag string
		// Author spans follow the title, so they attach to the last review.
		authorSeen int
		buf        string
		// Whether `buf` is currently collecting a title rather than an author.
		titleSink bool
	)

	z := html.NewTokenizer(r)
	for {
		switch z.Next() {
		case html.ErrorToken:
			if err := z.Err(); err != nil && err != io.EOF {
				slog.Error("telerama: ParseList tokenizer failed", "error", err)
				return nil, errors.New("failed to read telerama index")
			}
			// Drop anything without both of the fields we came for.
			keep := make([]Review, 0, len(out))
			for _, r := range out {
				r.Title = strings.TrimSpace(r.Title)
				if r.Title != "" && r.URL != "" {
					keep = append(keep, r)
				}
			}
			return keep, nil

		case html.TextToken:
			if sink != nil {
				*sink += string(z.Text())
			}

		case html.StartTagToken, html.SelfClosingTagToken:
			nameB, hasAttr := z.TagName()
			name := string(nameB)
			attrs := attrsOf(z, hasAttr)
			class := attrs["class"]
			switch {
			case name == "a" && hasClass(class, "search__card-content-title-link"):
				out = append(out, Review{URL: absURL(attrs["href"])})
				authorSeen = 0
				// Collect into a local, never into the slice: appending can
				// reallocate the backing array and leave the sink dangling.
				buf = ""
				sink, captureTag = &buf, name
				titleSink = true
			case name == "span" && hasClass(class, "search__card-author") && len(out) > 0:
				// First span is the director, second holds genre and year.
				buf = ""
				sink, captureTag = &buf, name
				titleSink = false
			}

		case html.EndTagToken:
			nameB, _ := z.TagName()
			if sink == nil || string(nameB) != captureTag {
				continue
			}
			if len(out) > 0 {
				cur := &out[len(out)-1]
				switch {
				case titleSink:
					cur.Title = strings.TrimSpace(buf)
				case authorSeen == 0:
					cur.Director = strings.TrimSpace(buf)
				case authorSeen == 1:
					if y := yearRe.FindString(buf); y != "" {
						cur.Year, _ = strconv.Atoi(y)
					}
				}
				if !titleSink {
					authorSeen++
				}
			}
			sink, captureTag, titleSink = nil, "", false
		}
	}
}

func absURL(href string) string {
	if href == "" || strings.HasPrefix(href, "http") {
		return href
	}
	return baseURL + href
}

func attrsOf(z *html.Tokenizer, hasAttr bool) map[string]string {
	m := map[string]string{}
	for hasAttr {
		var k, v []byte
		k, v, hasAttr = z.TagAttr()
		m[string(k)] = string(v)
	}
	return m
}

func hasClass(class, want string) bool {
	for _, c := range strings.Fields(class) {
		if c == want {
			return true
		}
	}
	return false
}
