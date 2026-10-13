package entity

import (
	"time"

	"github.com/sbondCo/Watcharr/database/dbmodel"
)

// LiveFriend is someone the user went to a show with (table `live_friends`).
// Fork-only, like LiveShow. Plain names, not Watcharr accounts. Identified by
// name per user, like LivePerson, and never deleted for the same reason.
type LiveFriend struct {
	dbmodel.GormModel
	UserID uint   `json:"-" gorm:"not null;uniqueIndex:idx_live_friends_user_key"`
	Name   string `json:"name" gorm:"not null"`
	// Same normalization as LivePerson.NameKey.
	NameKey string `json:"-" gorm:"not null;uniqueIndex:idx_live_friends_user_key"`
}

// LiveShowFriend links a show to a friend who was there (table
// `live_show_friends`). Kept when the show is soft-deleted, like LiveCredit.
type LiveShowFriend struct {
	ID       uint `gorm:"primarykey"`
	ShowID   uint `gorm:"not null;index"`
	FriendID uint `gorm:"not null;index"`
	// Position in the show's list, as entered.
	Ord int `gorm:"not null"`
}

// LivePhoto is a photo of a show, the ticket included (table `live_photos`).
// Hard-deleted: the image itself goes with the nightly image cleanup once no
// row points at it any more. Rows of a soft-deleted show stay, so their images
// are kept, like posters.
type LivePhoto struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	ShowID    uint `gorm:"not null;index"`
	ImageID   uint `gorm:"not null;index"`
	Image     *Image
	IsTicket  bool   `gorm:"not null;default:false"`
	Caption   string `gorm:"not null;default:''"`
	Ord       int    `gorm:"not null"`
}
