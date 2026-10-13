package live

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sbondCo/Watcharr/domain"
)

const (
	maxTitle       = 300
	maxVenue       = 200
	maxDescription = 5000
)

// ValidationError is a problem with the request itself, reported as a 400.
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

func invalid(format string, a ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, a...)}
}

// showFields is a request once trimmed, parsed and checked.
type showFields struct {
	Title       string
	Venue       string
	URL         string
	Description string
	Date        time.Time
	Missed      bool
	PosterURL   string
}

// validate trims and checks a request. Lengths count characters, not bytes:
// French titles are full of accents and must not hit the limit early.
func validate(r domain.LiveShowRequest) (showFields, error) {
	f := showFields{
		Title:       strings.TrimSpace(r.Title),
		Venue:       strings.TrimSpace(r.Venue),
		URL:         strings.TrimSpace(r.URL),
		Description: strings.TrimSpace(r.Description),
		Missed:      r.Missed,
		PosterURL:   strings.TrimSpace(r.PosterURL),
	}
	if f.Title == "" {
		return f, invalid("title is required")
	}
	if utf8.RuneCountInString(f.Title) > maxTitle {
		return f, invalid("title must be at most %d characters", maxTitle)
	}
	if utf8.RuneCountInString(f.Venue) > maxVenue {
		return f, invalid("venue must be at most %d characters", maxVenue)
	}
	if utf8.RuneCountInString(f.Description) > maxDescription {
		return f, invalid("description must be at most %d characters", maxDescription)
	}
	if f.URL != "" && !isAbsoluteHTTP(f.URL) {
		return f, invalid("url must be an absolute http or https link")
	}
	// Only the shape here: whether we may and can download it is decided
	// when fetching.
	if f.PosterURL != "" && !isAbsoluteHTTP(f.PosterURL) {
		return f, invalid("posterUrl must be an absolute http or https link")
	}
	date := strings.TrimSpace(r.Date)
	if date == "" {
		return f, invalid("date is required")
	}
	d, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return f, invalid("date must be an RFC3339 date-time (e.g. 2026-10-13T20:30:00+02:00)")
	}
	f.Date = d.UTC()
	return f, nil
}

func isAbsoluteHTTP(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
