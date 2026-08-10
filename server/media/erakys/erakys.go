// Package erakys reads cinema programming from sites built on the Erakys
// platform (CMS + TicketingCiné booking), which many French independent
// cinemas use.
//
// There is no public API: the site's showtimes page is rendered client-side by
// a jQuery call to /FR/ajax/cine/faisan.horairesjour, which answers with an
// HTML fragment. We call the same endpoint and parse the fragment.
//
// That makes this a scraper, with the usual consequence: it breaks when the
// template changes. It is written against the markup documented in
// testdata/day_fragment.html, and TestParseDay is what tells you it broke.
// Callers must cache — these are small cinemas, not a CDN.
package erakys

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

	"golang.org/x/net/html"
)

// Venue is one cinema served by an Erakys site.
type Venue struct {
	// Display name.
	Name string
	// Site host, without scheme, e.g. "lesvarietes-marseille.com".
	Host string
}

// Showtime is a single screening.
type Showtime struct {
	// Start time, "20:15" (local to the cinema).
	Start string `json:"start"`
	// End time, "22:19". Empty when the site doesn't publish it.
	End string `json:"end"`
	// "VO", "VOST" or "VF". Empty when not stated.
	Version string `json:"version"`
	// Deep link into the cinema's booking flow.
	BookingURL string `json:"bookingUrl"`
}

// Film is one film screened on a given day, with all of its showtimes.
type Film struct {
	// Erakys film code, e.g. "JM0J7". Unique per site, not across sites, and
	// unrelated to any TMDB id. Used here as the join key within a fragment.
	Code string `json:"code"`
	// Release title, as the cinema publishes it (usually upper case, French).
	Title string `json:"title"`
	// e.g. "Drame". Empty when not stated.
	Genre string `json:"genre"`
	// Running time in minutes, 0 when not stated.
	Runtime   int        `json:"runtime"`
	Showtimes []Showtime `json:"showtimes"`
}

const (
	// Path of the fragment endpoint, relative to a venue host.
	dayPath = "/FR/ajax/cine/faisan.horairesjour"
	// Identify ourselves rather than pretending to be a browser.
	userAgent = "Watcharr (self-hosted watch list; showtimes for the user's own cinemas)"
)

var httpClient = &http.Client{Timeout: 20 * time.Second}

// "01h55" or "1h55" -> minutes. Also matches "22h19" times.
var hourMinRe = regexp.MustCompile(`(\d{1,2})h(\d{2})`)

// FetchDay returns everything screening at `host` on `day`.
//
// The endpoint wants both the Monday of the week and the day itself; asking for
// a day outside the selected week yields that week instead, so both are derived
// from `day` here rather than taken from the caller.
func FetchDay(host string, day time.Time) ([]Film, error) {
	u := fmt.Sprintf(
		"https://%s%s?semaineSel=%s&jourSel=%s&afficherSM=false&isInDetail=false&filtres=",
		host, dayPath, mondayOf(day).Format(time.DateOnly), day.Format(time.DateOnly),
	)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Error("erakys: FetchDay request failed", "host", host, "error", err)
		return nil, errors.New("showtimes request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Error("erakys: FetchDay bad status", "host", host, "status", resp.StatusCode)
		return nil, fmt.Errorf("showtimes request returned %d", resp.StatusCode)
	}
	films, err := ParseDay(resp.Body)
	if err != nil {
		return nil, err
	}
	slog.Debug("erakys: FetchDay parsed",
		"host", host, "day", day.Format(time.DateOnly), "films", len(films))
	return films, nil
}

// ParseDay reads the HTML fragment returned by the day endpoint.
//
// Shape it relies on (see the testdata fixture):
//
//	<div class="film-card">                      per film
//	  ... data-film="JM0J7" ...                  join key, precedes the title
//	  <h3>GIRL</h3>
//	  <div class="genre-film"><i>Drame | 01h55</i></div>
//	  <div class="film-seances-jour">
//	    <a class="Erakys_select_seance" data-film="JM0J7" href="…ticketingcine…">
//	      <div class="heure">20h15</div>
//	      <div class="heureFin">(fin 22h19)</div>
//	      <div class="version VOST v-">VOST</div>
//	    </a>
//
// Films with no showtime left in the day are dropped: the site keeps rendering
// them as passive entries, and "showing today" should mean you can still go.
func ParseDay(r io.Reader) ([]Film, error) {
	var (
		order  []string
		byCode = make(map[string]*Film)
		// Last film code seen on any element; the card carries it before its
		// title and genre, so it attributes both.
		lastCode string
		// Non-nil while inside a booking anchor.
		show     *Showtime
		showCode string
		// Text sink: where TextTokens go, until captureTag closes.
		sink       *string
		captureTag string
	)
	film := func(code string) *Film {
		if f, ok := byCode[code]; ok {
			return f
		}
		f := &Film{Code: code}
		byCode[code] = f
		order = append(order, code)
		return f
	}

	z := html.NewTokenizer(r)
	for {
		switch z.Next() {
		case html.ErrorToken:
			if err := z.Err(); err != nil && err != io.EOF {
				slog.Error("erakys: ParseDay tokenizer failed", "error", err)
				return nil, errors.New("failed to read showtimes")
			}
			films := make([]Film, 0, len(order))
			for _, c := range order {
				if f := byCode[c]; f.Title != "" && len(f.Showtimes) > 0 {
					films = append(films, *f)
				}
			}
			return films, nil

		case html.TextToken:
			if sink != nil {
				*sink += string(z.Text())
			}

		case html.StartTagToken, html.SelfClosingTagToken:
			nameB, hasAttr := z.TagName()
			name := string(nameB)
			attrs := attrsOf(z, hasAttr)
			if c := attrs["data-film"]; c != "" {
				lastCode = c
			}
			class := attrs["class"]
			switch {
			case name == "a" && hasClass(class, "Erakys_select_seance"):
				showCode = attrs["data-film"]
				show = &Showtime{BookingURL: attrs["href"]}
			case name == "h2" || name == "h3":
				// Titles are rendered twice per card (desktop + mobile); first wins.
				if lastCode != "" && film(lastCode).Title == "" {
					sink, captureTag = &film(lastCode).Title, name
				}
			case hasClass(class, "genre-film"):
				if lastCode != "" && film(lastCode).Genre == "" {
					// Parsed on close, since it holds "Drame | 01h55".
					sink, captureTag = &film(lastCode).Genre, name
				}
			case show != nil && hasClass(class, "heure"):
				sink, captureTag = &show.Start, name
			case show != nil && hasClass(class, "heureFin"):
				sink, captureTag = &show.End, name
			case show != nil && hasClass(class, "version"):
				// Only inside an anchor: the card wraps its genre in a
				// `class="version"` div too, which is not a version at all.
				sink, captureTag = &show.Version, name
			}

		case html.EndTagToken:
			nameB, _ := z.TagName()
			name := string(nameB)
			if sink != nil && name == captureTag {
				sink, captureTag = nil, ""
			}
			if show != nil && name == "a" {
				if f := film(showCode); showCode != "" {
					show.Start = firstTime(show.Start)
					show.End = firstTime(show.End)
					show.Version = normalizeVersion(show.Version)
					if show.Start != "" {
						f.Showtimes = append(f.Showtimes, *show)
					}
				}
				show, showCode = nil, ""
			}
			if name == "h2" || name == "h3" {
				if f, ok := byCode[lastCode]; ok {
					f.Title = strings.TrimSpace(f.Title)
				}
			}
			if name == "div" {
				if f, ok := byCode[lastCode]; ok && f.Genre != "" && f.Runtime == 0 {
					f.Genre, f.Runtime = splitGenreRuntime(f.Genre)
				}
			}
		}
	}
}

// mondayOf returns the Monday of the ISO week containing t.
func mondayOf(t time.Time) time.Time {
	off := (int(t.Weekday()) + 6) % 7 // Monday=0 … Sunday=6
	return t.AddDate(0, 0, -off)
}

// firstTime turns "20h15" or "(fin 22h19)" into "20:15" / "22:19".
// Returns "" when there is no time in s.
func firstTime(s string) string {
	m := hourMinRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	h, _ := strconv.Atoi(m[1])
	return fmt.Sprintf("%02d:%s", h, m[2])
}

// splitGenreRuntime turns "Drame | 01h55" into ("Drame", 115).
func splitGenreRuntime(s string) (string, int) {
	s = strings.TrimSpace(s)
	runtime := 0
	if m := hourMinRe.FindStringSubmatch(s); m != nil {
		h, _ := strconv.Atoi(m[1])
		mins, _ := strconv.Atoi(m[2])
		runtime = h*60 + mins
		s = strings.Replace(s, m[0], "", 1)
	}
	return strings.Trim(strings.TrimSpace(s), "|-• \t"), runtime
}

// normalizeVersion keeps only the version label we understand.
func normalizeVersion(s string) string {
	switch v := strings.ToUpper(strings.TrimSpace(s)); {
	case strings.HasPrefix(v, "VOST"):
		return "VOST"
	case strings.HasPrefix(v, "VO"):
		return "VO"
	case strings.HasPrefix(v, "VF"):
		return "VF"
	default:
		return ""
	}
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
