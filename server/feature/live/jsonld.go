package live

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
	// Embedded so Europe/Paris resolves in the Alpine image, which has no
	// zoneinfo.
	_ "time/tzdata"

	"github.com/sbondCo/Watcharr/domain"
)

// maxPerformances caps the list sent to the form: a season page can list
// every date of a long run.
const maxPerformances = 60

// ldEvent is what we keep from one schema.org Event, raw (still to be
// cleaned).
type ldEvent struct {
	name        string
	description string
	location    string
	images      []string
	start, end  string
}

// parseJSONLD adds the events of one <script type="application/ld+json">.
// Sites often ship broken JSON-LD: an invalid block is skipped, never an error.
func parseJSONLD(raw []byte, out *[]ldEvent) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return
	}
	collectEvents(v, out)
}

// collectEvents walks the shapes JSON-LD comes in: one object, an array of
// them, or a @graph.
func collectEvents(v any, out *[]ldEvent) {
	switch v := v.(type) {
	case []any:
		for _, e := range v {
			collectEvents(e, out)
		}
	case map[string]any:
		if g, ok := v["@graph"]; ok {
			collectEvents(g, out)
		}
		if isEventType(v["@type"]) {
			*out = append(*out, ldEvent{
				name:        ldString(v["name"]),
				description: ldString(v["description"]),
				location:    ldLocationName(v["location"]),
				images:      ldImages(v["image"], nil),
				start:       ldString(v["startDate"]),
				end:         ldString(v["endDate"]),
			})
		}
	}
}

// isEventType accepts Event and its subtypes (TheaterEvent, DanceEvent...),
// as a string or an array, bare or as a full schema.org IRI.
func isEventType(t any) bool {
	switch t := t.(type) {
	case string:
		if i := strings.LastIndexAny(t, "/:#"); i >= 0 {
			t = t[i+1:]
		}
		return strings.HasSuffix(t, "Event") || t == "Festival"
	case []any:
		for _, e := range t {
			if isEventType(e) {
				return true
			}
		}
	}
	return false
}

func ldString(v any) string {
	s, _ := v.(string)
	return s
}

// ldLocationName takes the first named place. The type key is not checked:
// some sites write "type" instead of "@type".
func ldLocationName(v any) string {
	switch v := v.(type) {
	case map[string]any:
		if s := cleanText(ldString(v["name"])); s != "" {
			return s
		}
	case []any:
		for _, e := range v {
			if s := ldLocationName(e); s != "" {
				return s
			}
		}
	}
	return ""
}

// ldImages flattens an image given as a URL, an ImageObject or an array.
func ldImages(v any, acc []string) []string {
	switch v := v.(type) {
	case string:
		acc = append(acc, v)
	case map[string]any:
		acc = append(acc, ldString(v["url"]), ldString(v["contentUrl"]))
	case []any:
		for _, e := range v {
			acc = ldImages(e, acc)
		}
	}
	return acc
}

var paris = func() *time.Location {
	if loc, err := time.LoadLocation("Europe/Paris"); err == nil {
		return loc
	}
	return time.UTC
}()

// Layouts carrying their own offset; RFC3339 also accepts fractional seconds.
var offsetLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04Z07:00",
	"2006-01-02T15:04:05Z0700",
}

// Offset-less times are taken as Paris local time: the theatres we read are
// there, and their pages print the hall's clock.
var localLayouts = []string{
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
}

// parseLDDate reads a schema.org date or date-time. hasTime is false for a
// bare day, which says nothing about the performance's hour.
func parseLDDate(s string) (t time.Time, hasTime, ok bool) {
	s = strings.TrimSpace(s)
	for _, l := range offsetLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true, true
		}
	}
	for _, l := range localLayouts {
		if t, err := time.ParseInLocation(l, s, paris); err == nil {
			return t, true, true
		}
	}
	if t, err := time.ParseInLocation(time.DateOnly, s, paris); err == nil {
		return t, false, true
	}
	return time.Time{}, false, false
}

// eventDates turns the events' dates into performances (when they have a
// time) or, failing any, the span of days they cover.
func eventDates(events []ldEvent) ([]string, *domain.LiveDateRange) {
	var times []time.Time
	seen := map[int64]bool{}
	var rng *domain.LiveDateRange
	for _, e := range events {
		start, hasTime, ok := parseLDDate(e.start)
		if !ok {
			continue
		}
		if hasTime {
			if !seen[start.Unix()] {
				seen[start.Unix()] = true
				times = append(times, start)
			}
			continue
		}
		first := start.Format(time.DateOnly)
		last := first
		if end, _, ok := parseLDDate(e.end); ok {
			// YYYY-MM-DD compares as text.
			if d := end.Format(time.DateOnly); d > last {
				last = d
			}
		}
		if rng == nil {
			rng = &domain.LiveDateRange{Start: first, End: last}
			continue
		}
		if first < rng.Start {
			rng.Start = first
		}
		if last > rng.End {
			rng.End = last
		}
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	if len(times) > maxPerformances {
		times = times[:maxPerformances]
	}
	perfs := make([]string, len(times))
	for i, t := range times {
		perfs[i] = t.Format(time.RFC3339)
	}
	if len(perfs) > 0 {
		// Exact performances already say more than a span of days.
		rng = nil
	}
	return perfs, rng
}
