package live

import (
	"encoding/json"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"gorm.io/gorm"
)

const (
	maxRating     = 10
	maxComment    = 10000
	maxFriendName = 100
	maxFriends    = 30
)

// souvenirFields is a PATCH body once checked. nil: key absent, leave as is.
type souvenirFields struct {
	// Non-nil inner pointer: the new rating; nil inner: clear it.
	Rating  **float64
	Comment *string
	Friends *[]string
}

// validateSouvenir reads a partial body: only the keys present are applied,
// so the client can save the rating without resending the comment. A null
// rating, comment or friends list clears it.
func validateSouvenir(body map[string]json.RawMessage) (souvenirFields, error) {
	var f souvenirFields
	if raw, ok := body["rating"]; ok {
		var r *float64
		if err := json.Unmarshal(raw, &r); err != nil {
			return f, invalid("rating must be a number or null")
		}
		if r != nil {
			if *r < 0 || *r > maxRating {
				return f, invalid("rating must be between 0 and %d", maxRating)
			}
			// One decimal, like the rest of Watcharr. A rating rounding to
			// 0 is no rating: 0 means "not rated".
			v := math.Round(*r*10) / 10
			if v == 0 {
				r = nil
			} else {
				r = &v
			}
		}
		f.Rating = &r
	}
	if raw, ok := body["comment"]; ok {
		var c *string
		if err := json.Unmarshal(raw, &c); err != nil {
			return f, invalid("comment must be a string")
		}
		// Trimmed only: line breaks inside are the user's layout.
		v := ""
		if c != nil {
			v = strings.TrimSpace(*c)
		}
		if utf8.RuneCountInString(v) > maxComment {
			return f, invalid("comment must be at most %d characters", maxComment)
		}
		f.Comment = &v
	}
	if raw, ok := body["friends"]; ok {
		var in []string
		if err := json.Unmarshal(raw, &in); err != nil {
			return f, invalid("friends must be a list of names")
		}
		if len(in) > maxFriends {
			return f, invalid("at most %d friends", maxFriends)
		}
		names := make([]string, 0, len(in))
		for i, n := range in {
			name := collapseSpaces(n)
			if name == "" {
				return f, invalid("friends[%d]: name is required", i)
			}
			if utf8.RuneCountInString(name) > maxFriendName {
				return f, invalid("friends[%d]: name must be at most %d characters", i, maxFriendName)
			}
			names = append(names, name)
		}
		f.Friends = &names
	}
	return f, nil
}

// UpdateSouvenir writes what the user kept of a show: rating, comment and who
// came. Separate from Update so editing the show's details can never wipe
// them, and the reverse.
func (s *Service) UpdateSouvenir(userId, id uint, body map[string]json.RawMessage) (domain.LiveShow, error) {
	f, err := validateSouvenir(body)
	if err != nil {
		return domain.LiveShow{}, err
	}
	e, err := s.find(userId, id)
	if err != nil {
		return domain.LiveShow{}, err
	}
	// updated_at always moves, friends-only changes included.
	cols := map[string]any{"updated_at": time.Now()}
	if f.Rating != nil {
		e.Rating = *f.Rating
		cols["rating"] = e.Rating
	}
	if f.Comment != nil {
		e.Comment = *f.Comment
		cols["comment"] = e.Comment
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Map, not struct: a nil rating must be written to clear it.
		if err := tx.Model(&entity.LiveShow{}).Where("id = ?", e.ID).Updates(cols).Error; err != nil {
			return err
		}
		if f.Friends != nil {
			return replaceFriends(tx, userId, e.ID, *f.Friends)
		}
		return nil
	})
	if err != nil {
		slog.Error("live: UpdateSouvenir failed", "user_id", userId, "id", id, "error", err)
		return domain.LiveShow{}, err
	}
	// Reloaded for the fresh updated_at.
	return s.Get(userId, id)
}

// replaceFriends writes a show's whole friend list, in the given order,
// creating the friends not met before.
func replaceFriends(tx *gorm.DB, userId, showId uint, names []string) error {
	if err := tx.Where("show_id = ?", showId).Delete(&entity.LiveShowFriend{}).Error; err != nil {
		return err
	}
	seen := map[string]bool{}
	rows := make([]entity.LiveShowFriend, 0, len(names))
	for _, name := range names {
		key := nameKey(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		var fr entity.LiveFriend
		res := tx.Where("user_id = ? AND name_key = ?", userId, key).Limit(1).Find(&fr)
		if res.Error != nil {
			return res.Error
		}
		// An existing friend keeps their stored spelling.
		if res.RowsAffected == 0 {
			fr = entity.LiveFriend{UserID: userId, Name: name, NameKey: key}
			if err := tx.Create(&fr).Error; err != nil {
				return err
			}
		}
		rows = append(rows, entity.LiveShowFriend{ShowID: showId, FriendID: fr.ID, Ord: len(rows)})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

// friendsFor loads the friends of several shows in one query; every show gets
// a non-nil slice.
func (s *Service) friendsFor(showIds []uint) (map[uint][]domain.LiveFriend, error) {
	out := make(map[uint][]domain.LiveFriend, len(showIds))
	for _, id := range showIds {
		out[id] = []domain.LiveFriend{}
	}
	if len(showIds) == 0 {
		return out, nil
	}
	var rows []struct {
		ShowID   uint
		FriendID uint
		Name     string
	}
	err := s.db.Table("live_show_friends AS sf").
		Select("sf.show_id, sf.friend_id, f.name").
		Joins("JOIN live_friends AS f ON f.id = sf.friend_id AND f.deleted_at IS NULL").
		Where("sf.show_id IN ?", showIds).
		Order("sf.show_id, sf.ord, sf.id").
		Scan(&rows).Error
	if err != nil {
		slog.Error("live: friendsFor failed", "error", err)
		return nil, err
	}
	for _, r := range rows {
		out[r.ShowID] = append(out[r.ShowID], domain.LiveFriend{ID: r.FriendID, Name: r.Name})
	}
	return out, nil
}

// Friends lists the user's friends who came to at least one show still in the
// notebook, most frequent first, for the suggestions. q, if not blank, filters
// on the name ignoring case and accents.
func (s *Service) Friends(userId uint, q string) ([]domain.LiveFriendSummary, error) {
	var rows []domain.LiveFriendSummary
	err := s.db.Table("live_friends AS f").
		Select("f.id, f.name, COUNT(DISTINCT sf.show_id) AS show_count").
		Joins("JOIN live_show_friends AS sf ON sf.friend_id = f.id").
		Joins("JOIN live_shows AS s ON s.id = sf.show_id AND s.user_id = f.user_id AND s.deleted_at IS NULL").
		Where("f.user_id = ? AND f.deleted_at IS NULL", userId).
		Group("f.id").
		Scan(&rows).Error
	if err != nil {
		slog.Error("live: Friends failed", "user_id", userId, "error", err)
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.ShowCount != b.ShowCount {
			return a.ShowCount > b.ShowCount
		}
		if la, lb := strings.ToLower(a.Name), strings.ToLower(b.Name); la != lb {
			return la < lb
		}
		return a.ID < b.ID
	})
	out := make([]domain.LiveFriendSummary, 0, len(rows))
	fq := searchFold(q)
	for _, r := range rows {
		if fq != "" {
			if len(out) == maxSearchResults {
				break
			}
			if !strings.Contains(searchFold(r.Name), fq) {
				continue
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// decodeSouvenirBody keeps the raw value of each key, so "absent" and "null"
// stay distinct.
func decodeSouvenirBody(b []byte) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return nil, invalid("invalid request body")
	}
	return m, nil
}
