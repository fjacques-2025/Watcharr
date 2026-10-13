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
