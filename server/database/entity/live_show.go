package entity

import (
	"time"

	"github.com/sbondCo/Watcharr/database/dbmodel"
)

// LiveShow is one performance (theatre, dance...) in the user's notebook.
//
// Fork-only: it lives in its own `live_` table and nothing upstream references
// it, so an older image simply ignores the table and rolling back stays a plain
// image swap.
type LiveShow struct {
	dbmodel.GormModel
	UserID      uint   `json:"-" gorm:"not null;index"`
	Title       string `json:"title" gorm:"not null"`
	Venue       string `json:"venue"`
	URL         string `json:"url"`
	Description string `json:"description"`
	// Date and time of the performance the user booked / attended.
	// Stored in UTC: SQLite compares these as text, so mixed offsets would
	// sort wrongly.
	Date time.Time `json:"date" gorm:"not null;index"`
	// User correction: they did not go (cancelled, missed...).
	Missed bool `json:"missed" gorm:"not null;default:false"`
	// Where the poster was downloaded from, to skip downloading it again
	// when the show is saved unchanged.
	PosterURL string `json:"posterUrl"`
	// Local copy of the poster: venue pages disappear at the end of the
	// season. Points into the shared images table, like Game.Poster.
	PosterID *uint  `json:"-"`
	Poster   *Image `json:"poster,omitempty"`
}
