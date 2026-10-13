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
	maxPersonName  = 200
	maxRole        = 100
	maxCharacter   = 200
	maxCredits     = 100
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
	// nil: leave the credits as they are.
	Credits *[]creditFields
}

type creditFields struct {
	Name      string
	Role      string
	TmdbID    *int
	Character string
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
	if r.Credits != nil {
		cs, err := validateCredits(*r.Credits)
		if err != nil {
			return f, err
		}
		f.Credits = &cs
	}
	return f, nil
}

func validateCredits(in []domain.LiveCreditRequest) ([]creditFields, error) {
	if len(in) > maxCredits {
		return nil, invalid("at most %d credits", maxCredits)
	}
	out := make([]creditFields, 0, len(in))
	for i, c := range in {
		name, err := validPersonName(c.Name)
		if err != nil {
			return nil, invalid("credits[%d]: %s", i, err.Error())
		}
		role := collapseSpaces(c.Role)
		if role == "" {
			return nil, invalid("credits[%d]: role is required", i)
		}
		if utf8.RuneCountInString(role) > maxRole {
			return nil, invalid("credits[%d]: role must be at most %d characters", i, maxRole)
		}
		if err := validTmdbID(c.TmdbPersonID); err != nil {
			return nil, invalid("credits[%d]: %s", i, err.Error())
		}
		character := collapseSpaces(c.Character)
		if utf8.RuneCountInString(character) > maxCharacter {
			return nil, invalid("credits[%d]: character must be at most %d characters", i, maxCharacter)
		}
		out = append(out, creditFields{Name: name, Role: role, TmdbID: c.TmdbPersonID, Character: character})
	}
	return out, nil
}

func validPersonName(raw string) (string, error) {
	name := collapseSpaces(raw)
	if name == "" {
		return "", invalid("name is required")
	}
	if utf8.RuneCountInString(name) > maxPersonName {
		return "", invalid("name must be at most %d characters", maxPersonName)
	}
	return name, nil
}

func validTmdbID(id *int) error {
	if id != nil && *id <= 0 {
		return invalid("tmdbPersonId must be a positive integer")
	}
	return nil
}

// collapseSpaces trims and turns every run of whitespace into one space, so
// "Jean  Dupont" typed twice is the same artist.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func isAbsoluteHTTP(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
