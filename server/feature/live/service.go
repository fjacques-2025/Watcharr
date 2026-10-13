// Package live is the user's notebook of live shows (theatre, dance...).
//
// Step 1: shows entered by hand. Step 2: prefill from the show's page and a
// local copy of the poster. Credits, friends and photos come later, in their
// own `live_` tables.
package live

import (
	"errors"
	"log/slog"
	"time"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrNotFound covers both a missing show and another user's show: telling
// them apart would leak which ids exist.
var ErrNotFound = errors.New("live show not found")

type Service struct {
	db *gorm.DB
	// Clock for the computed state, swappable in tests.
	now func() time.Time
	// The only way out to the internet for this feature (pages, posters).
	fetch *fetcher
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, now: time.Now, fetch: newFetcher()}
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
		PosterURL:   e.PosterURL,
		Poster:      e.Poster,
	}
}

// List returns all of the user's shows, oldest performance first.
func (s *Service) List(userId uint) ([]domain.LiveShow, error) {
	var rows []entity.LiveShow
	// id breaks ties so two shows at the same time keep a stable order.
	if res := s.db.Preload("Poster").Where("user_id = ?", userId).
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
	res := s.db.Preload("Poster").Where("id = ? AND user_id = ?", id, userId).Take(&e)
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
	var poster []byte
	if f.PosterURL != "" {
		if poster, err = s.downloadPoster(f.PosterURL); err != nil {
			return domain.LiveShow{}, err
		}
	}
	e := entity.LiveShow{
		UserID:      userId,
		Title:       f.Title,
		Venue:       f.Venue,
		URL:         f.URL,
		Description: f.Description,
		Date:        f.Date,
		PosterURL:   f.PosterURL,
	}
	// One transaction: a poster the saver rejects must not leave a show
	// behind, nor a show fail after its image row was added.
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if poster != nil {
			if err := attachPoster(tx, &e, poster); err != nil {
				return err
			}
		}
		// The image row is written by the saver; GORM must not upsert it.
		return tx.Omit(clause.Associations).Create(&e).Error
	})
	if err != nil {
		slog.Error("live: Create failed", "user_id", userId, "error", err)
		return domain.LiveShow{}, err
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

	// Same source as stored: keep the copy we have. The image is never
	// deleted here, other rows may share it (the saver dedups by hash).
	var poster []byte
	switch {
	case f.PosterURL == "":
		e.PosterID, e.Poster = nil, nil
	case f.PosterURL == e.PosterURL && e.PosterID != nil:
	default:
		if poster, err = s.downloadPoster(f.PosterURL); err != nil {
			return domain.LiveShow{}, err
		}
	}
	e.PosterURL = f.PosterURL

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if poster != nil {
			if err := attachPoster(tx, e, poster); err != nil {
				return err
			}
		}
		// Save writes zero values too, which a full replace needs (clearing
		// the venue, un-marking missed, dropping the poster).
		return tx.Omit(clause.Associations).Save(e).Error
	})
	if err != nil {
		slog.Error("live: Update failed", "user_id", userId, "id", id, "error", err)
		return domain.LiveShow{}, err
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
