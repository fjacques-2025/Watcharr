package live

import (
	"io"
	"log/slog"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"gorm.io/gorm"
)

// Posters the user provides rather than one a page declares: some pages have
// none, and on a phone copying an image's address is next to impossible.
// They have no source URL, so poster_url is cleared — which Update reads as
// "keep the poster we have" when the form sends "" back.

// UploadPoster stores the image as the show's poster.
func (s *Service) UploadPoster(userId, showId uint, file io.Reader) (domain.LiveShow, error) {
	e, err := s.find(userId, showId)
	if err != nil {
		return domain.LiveShow{}, err
	}
	b, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		slog.Error("live: UploadPoster read failed", "error", err)
		return domain.LiveShow{}, err
	}
	if len(b) > maxImageBytes {
		return domain.LiveShow{}, posterError("file is too big")
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := attachPoster(tx, e, b); err != nil {
			return err
		}
		return setPoster(tx, e.ID, e.PosterID)
	})
	if err != nil {
		return domain.LiveShow{}, err
	}
	return s.reloaded(userId, showId)
}

// PosterFromPhoto uses one of the show's photos — a snap of the poster or of
// the programme — as its poster. The image is shared, not copied: it stays
// referenced by the poster even if the photo is deleted.
func (s *Service) PosterFromPhoto(userId, showId, photoId uint) (domain.LiveShow, error) {
	p, err := s.findPhoto(userId, showId, photoId)
	if err != nil {
		return domain.LiveShow{}, err
	}
	imageId := p.ImageID
	if err := setPoster(s.db, showId, &imageId); err != nil {
		return domain.LiveShow{}, err
	}
	return s.reloaded(userId, showId)
}

// RemovePoster drops the poster, whatever its origin. The image itself is
// left to the nightly cleanup once nothing references it.
func (s *Service) RemovePoster(userId, showId uint) (domain.LiveShow, error) {
	if _, err := s.find(userId, showId); err != nil {
		return domain.LiveShow{}, err
	}
	if err := setPoster(s.db, showId, nil); err != nil {
		return domain.LiveShow{}, err
	}
	return s.reloaded(userId, showId)
}

func setPoster(db *gorm.DB, showId uint, imageId *uint) error {
	err := db.Model(&entity.LiveShow{}).Where("id = ?", showId).
		Updates(map[string]any{"poster_id": imageId, "poster_url": ""}).Error
	if err != nil {
		slog.Error("live: setPoster failed", "show_id", showId, "error", err)
	}
	return err
}

func (s *Service) reloaded(userId, showId uint) (domain.LiveShow, error) {
	e, err := s.find(userId, showId)
	if err != nil {
		return domain.LiveShow{}, err
	}
	return s.toDtoWithCredits(e)
}
