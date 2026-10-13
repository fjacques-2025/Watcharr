package domain

import (
	"time"

	"github.com/sbondCo/Watcharr/database/entity"
)

// State of a live show. Computed from the date on every read, never stored, so
// a booked show becomes "seen" on its own once the performance has passed.
type LiveShowState string

const (
	LiveShowUpcoming LiveShowState = "upcoming"
	LiveShowSeen     LiveShowState = "seen"
	LiveShowMissed   LiveShowState = "missed"
)

// Body of a create/update. Date is a string rather than time.Time so a bad
// value gets our own clear message instead of a JSON decoding error.
type LiveShowRequest struct {
	Title       string `json:"title"`
	Venue       string `json:"venue"`
	URL         string `json:"url"`
	Description string `json:"description"`
	// RFC3339 date and time of the performance.
	Date string `json:"date"`
	// Ignored on create: a new show cannot already have been missed.
	Missed bool `json:"missed"`
	// Where to download the poster from. Empty: no poster. Same as the stored
	// one: kept without downloading again.
	PosterURL string `json:"posterUrl"`
	// Absent or null: credits unchanged (none on create). Present, even
	// empty: replaces them all, in this order.
	Credits *[]LiveCreditRequest `json:"credits"`
}

type LiveCreditRequest struct {
	Name string `json:"name"`
	Role string `json:"role"`
	// Set when the user picked a TMDB person; links the artist to it.
	TmdbPersonID *int `json:"tmdbPersonId"`
}

type LiveShow struct {
	ID          uint          `json:"id"`
	Title       string        `json:"title"`
	Venue       string        `json:"venue"`
	URL         string        `json:"url"`
	Description string        `json:"description"`
	Date        time.Time     `json:"date"`
	Missed      bool          `json:"missed"`
	State       LiveShowState `json:"state"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	// Source the poster was copied from; the copy itself is Poster.
	PosterURL string        `json:"posterUrl"`
	Poster    *entity.Image `json:"poster,omitempty"`
	// In entry order. Never null.
	Credits []LiveCredit `json:"credits"`
}

type LiveCredit struct {
	PersonID     uint   `json:"personId"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	TmdbPersonID *int   `json:"tmdbPersonId"`
}

type LivePerson struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	TmdbPersonID *int   `json:"tmdbPersonId"`
}

// An artist in the list, with the number of (non-deleted) shows they are in.
type LivePersonSummary struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	TmdbPersonID *int   `json:"tmdbPersonId"`
	ShowCount    int    `json:"showCount"`
	// Distinct roles held in the user's non-deleted shows, alphabetical (the
	// client orders them for display). Never null.
	Roles []string `json:"roles" gorm:"-"`
}

// An artist's page: their shows grouped by role. Never null.
type LivePersonDetail struct {
	ID           uint             `json:"id"`
	Name         string           `json:"name"`
	TmdbPersonID *int             `json:"tmdbPersonId"`
	Roles        []LivePersonRole `json:"roles"`
}

type LivePersonRole struct {
	Role string `json:"role"`
	// Most recent first.
	Shows []LiveShow `json:"shows"`
}

// Null person: none of the user's artists is linked to this TMDB person, or
// none of their shows is left.
type LivePersonByTmdb struct {
	Person *LivePersonDetail `json:"person"`
}

// Rename an artist and link/unlink TMDB (null unlinks).
type LivePersonUpdateRequest struct {
	Name         string `json:"name"`
	TmdbPersonID *int   `json:"tmdbPersonId"`
}

// Fields read from a show's web page, to prefill the form. The text fields are
// strings, empty when the page had nothing, and fit the validation limits.
type LivePrefill struct {
	// After redirects.
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Venue       string `json:"venue"`
	// Absolute; not downloaded until the show is saved with it.
	PosterURL string `json:"posterUrl"`
	// RFC3339 date-times of the performances the page lists, ascending,
	// deduplicated, past ones included. Never null: empty when none.
	Performances []string `json:"performances"`
	// Set only when the page gives days without a time, so the user still
	// has to pick the performance; nil otherwise.
	DateRange *LiveDateRange `json:"dateRange"`
}

// Days as YYYY-MM-DD, both inclusive.
type LiveDateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type LivePrefillRequest struct {
	URL string `json:"url"`
}
