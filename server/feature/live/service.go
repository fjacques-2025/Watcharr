// Package live is the user's notebook of live shows (theatre, dance...).
//
// Step 1 of the feature: shows only, entered by hand. Links, credits, friends
// and photos come later, in their own `live_` tables.
package live

import (
	"errors"
	"log/slog"
	"time"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"gorm.io/gorm"
)

// ErrNotFound covers both a missing show and another user's show: telling
// them apart would leak which ids exist.
var ErrNotFound = errors.New("live show not found")

type Service struct {
	db *gorm.DB
	// Clock for the computed state, swappable in tests.
	now func() time.Time
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, now: time.Now}
}

func (s *Service) toDto(e *entity.LiveShow, now time.Time) domain.LiveShow {
	return domain.LiveShow{
		ID:          e.ID,
		Title:       e.Title,
		Venue:       e.Venue,
		URL:         e.URL,
		Description: e.Description,
		Date:        e.Date,
		Missed:      e.Missed,
		State:       ShowState(e.Missed, e.Date, now),
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

// List returns all of the user's shows, oldest performance first.
func (s *Service) List(userId uint) ([]domain.LiveShow, error) {
	var rows []entity.LiveShow
	// id breaks ties so two shows at the same time keep a stable order.
	if res := s.db.Where("user_id = ?", userId).
		Order("date ASC, id ASC").
		Find(&rows); res.Error != nil {
		slog.Error("live: List failed", "user_id", userId, "error", res.Error)
		return nil, res.Error
	}
	now := s.now()
	out := make([]domain.LiveShow, 0, len(rows))
	for i := range rows {
		out = append(out, s.toDto(&rows[i], now))
	}
	return out, nil
}

func (s *Service) find(userId, id uint) (*entity.LiveShow, error) {
	var e entity.LiveShow
	res := s.db.Where("id = ? AND user_id = ?", id, userId).Take(&e)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		slog.Error("live: find failed", "user_id", userId, "id", id, "error", res.Error)
		return nil, res.Error
	}
	return &e, nil
}

func (s *Service) Get(userId, id uint) (domain.LiveShow, error) {
	e, err := s.find(userId, id)
	if err != nil {
		return domain.LiveShow{}, err
	}
	return s.toDto(e, s.now()), nil
}

func (s *Service) Create(userId uint, r domain.LiveShowRequest) (domain.LiveShow, error) {
	f, err := validate(r)
	if err != nil {
		return domain.LiveShow{}, err
	}
	e := entity.LiveShow{
		UserID:      userId,
		Title:       f.Title,
		Venue:       f.Venue,
		URL:         f.URL,
		Description: f.Description,
		Date:        f.Date,
	}
	if res := s.db.Create(&e); res.Error != nil {
		slog.Error("live: Create failed", "user_id", userId, "error", res.Error)
		return domain.LiveShow{}, res.Error
	}
	return s.toDto(&e, s.now()), nil
}

// Update replaces every editable field, `missed` included.
func (s *Service) Update(userId, id uint, r domain.LiveShowRequest) (domain.LiveShow, error) {
	f, err := validate(r)
	if err != nil {
		return domain.LiveShow{}, err
	}
	e, err := s.find(userId, id)
	if err != nil {
		return domain.LiveShow{}, err
	}
	e.Title = f.Title
	e.Venue = f.Venue
	e.URL = f.URL
	e.Description = f.Description
	e.Date = f.Date
	e.Missed = f.Missed
	// Save writes zero values too, which a full replace needs (clearing the
	// venue, un-marking missed).
	if res := s.db.Save(e); res.Error != nil {
		slog.Error("live: Update failed", "user_id", userId, "id", id, "error", res.Error)
		return domain.LiveShow{}, res.Error
	}
	return s.toDto(e, s.now()), nil
}

func (s *Service) Delete(userId, id uint) error {
	res := s.db.Where("id = ? AND user_id = ?", id, userId).Delete(&entity.LiveShow{})
	if res.Error != nil {
		slog.Error("live: Delete failed", "user_id", userId, "id", id, "error", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
