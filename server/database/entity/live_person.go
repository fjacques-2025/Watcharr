package entity

import "github.com/sbondCo/Watcharr/database/dbmodel"

// LivePerson is an artist credited in the user's live shows (table
// `live_people`). Fork-only, like LiveShow.
//
// Identified by name, per user: the stage world has no shared catalogue, and
// most artists are not on TMDB. Never deleted, so the unique index cannot
// collide with a soft-deleted row.
type LivePerson struct {
	dbmodel.GormModel
	UserID uint   `json:"-" gorm:"not null;uniqueIndex:idx_live_people_user_key"`
	Name   string `json:"name" gorm:"not null"`
	// Trimmed, whitespace-collapsed, lowercased name. Accents are kept:
	// "Élise" and "Elise" may be two people, the user decides through the
	// suggestions.
	NameKey string `json:"-" gorm:"not null;uniqueIndex:idx_live_people_user_key"`
	// Optional link to the TMDB person, always set by the user, never guessed:
	// homonyms are common.
	TmdbPersonID *int `json:"tmdbPersonId" gorm:"index"`
}

// LiveCredit is one "person + role" line of a show (table `live_credits`).
// Kept when the show is soft-deleted: reads filter on the show instead.
type LiveCredit struct {
	ID       uint   `gorm:"primarykey"`
	ShowID   uint   `gorm:"not null;index"`
	PersonID uint   `gorm:"not null;index"`
	Role     string `gorm:"not null"`
	// Part played, "" when none. Column renamed: CHARACTER is an SQL type.
	Character string `gorm:"column:character_name;not null;default:''"`
	// Position in the show's credits, as entered.
	Ord int `gorm:"not null"`
}
