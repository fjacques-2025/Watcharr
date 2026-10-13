package domain

import "time"

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
}
