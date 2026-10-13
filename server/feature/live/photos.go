package live

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"github.com/sbondCo/Watcharr/image"
	"github.com/sbondCo/Watcharr/util"
	"gorm.io/gorm"
)

// ErrPhotoNotFound covers a photo of another show or another user too.
var ErrPhotoNotFound = errors.New("photo not found")

const (
	// Under img/live-photos/, mirrored by the backup with the rest of img/.
	photoSubPath    = "live-photos"
	maxPhotosByShow = 50
	maxCaption      = 300
	// The saver takes 10 MiB of image; the rest is multipart overhead. The
	// client shrinks photos to 2048 px before sending, so this is generous.
	maxPhotoBodyBytes = 12 * util.Mebibyte
)

func photoError(reason string) error {
	return invalid("could not use that photo: %s", reason)
}

func validCaption(raw string) (string, error) {
	c := strings.TrimSpace(raw)
	if utf8.RuneCountInString(c) > maxCaption {
		return "", invalid("caption must be at most %d characters", maxCaption)
	}
	return c, nil
}

func photoDto(p *entity.LivePhoto) domain.LivePhoto {
	d := domain.LivePhoto{ID: p.ID, IsTicket: p.IsTicket, Caption: p.Caption}
	if p.Image != nil {
		d.Image = *p.Image
	}
	return d
}

// AddPhoto validates, re-encodes and stores a photo through the shared image
// saver (format, size and pixel limits, dedup by hash), then adds it to the
// show, last.
func (s *Service) AddPhoto(userId, showId uint, file io.Reader, isTicket bool, caption string) (domain.LivePhoto, error) {
	caption, err := validCaption(caption)
	if err != nil {
		return domain.LivePhoto{}, err
	}
	if _, err := s.find(userId, showId); err != nil {
		return domain.LivePhoto{}, err
	}
	var p entity.LivePhoto
	// One transaction: the limit and the next ord are read where they are
	// used, and a rejected image leaves no row behind (the saver checks the
	// image before writing anything).
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&entity.LivePhoto{}).Where("show_id = ?", showId).Count(&n).Error; err != nil {
			return err
		}
		if n >= maxPhotosByShow {
			return invalid("at most %d photos per show", maxPhotosByShow)
		}
		var maxOrd int
		if err := tx.Model(&entity.LivePhoto{}).Where("show_id = ?", showId).
			Select("COALESCE(MAX(ord), 0)").Scan(&maxOrd).Error; err != nil {
			return err
		}
		img, err := image.NewSaver(tx, photoSubPath, image.ValidateOptions{}).DownloadAndInsert(file)
		if err != nil {
			if reason, ok := saverRejections[err.Error()]; ok {
				return photoError(reason)
			}
			return fmt.Errorf("storing photo: %w", err)
		}
		p = entity.LivePhoto{
			ShowID: showId, ImageID: img.ID, Image: &img,
			IsTicket: isTicket, Caption: caption, Ord: maxOrd + 1,
		}
		// The image row is written by the saver; GORM must not upsert it.
		return tx.Omit("Image").Create(&p).Error
	})
	if err != nil {
		logUnlessInvalid("live: AddPhoto failed", err, "user_id", userId, "show_id", showId)
		return domain.LivePhoto{}, err
	}
	return photoDto(&p), nil
}

// findPhoto loads a photo of one of the user's (non-deleted) shows.
func (s *Service) findPhoto(userId, showId, photoId uint) (*entity.LivePhoto, error) {
	if _, err := s.find(userId, showId); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrPhotoNotFound
		}
		return nil, err
	}
	var p entity.LivePhoto
	res := s.db.Preload("Image").Where("id = ? AND show_id = ?", photoId, showId).Limit(1).Find(&p)
	if res.Error != nil {
		slog.Error("live: findPhoto failed", "user_id", userId, "id", photoId, "error", res.Error)
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrPhotoNotFound
	}
	return &p, nil
}

func (s *Service) UpdatePhoto(userId, showId, photoId uint, r domain.LivePhotoUpdateRequest) (domain.LivePhoto, error) {
	caption, err := validCaption(r.Caption)
	if err != nil {
		return domain.LivePhoto{}, err
	}
	p, err := s.findPhoto(userId, showId, photoId)
	if err != nil {
		return domain.LivePhoto{}, err
	}
	// Map, not struct: false and "" must be written.
	if err := s.db.Model(&entity.LivePhoto{}).Where("id = ?", p.ID).
		Updates(map[string]any{"is_ticket": r.IsTicket, "caption": caption}).Error; err != nil {
		slog.Error("live: UpdatePhoto failed", "user_id", userId, "id", photoId, "error", err)
		return domain.LivePhoto{}, err
	}
	p.IsTicket, p.Caption = r.IsTicket, caption
	return photoDto(p), nil
}

// DeletePhoto removes the row only: the image may be shared (dedup by hash),
// the nightly image cleanup deletes it once nothing points at it.
func (s *Service) DeletePhoto(userId, showId, photoId uint) error {
	p, err := s.findPhoto(userId, showId, photoId)
	if err != nil {
		return err
	}
	if err := s.db.Delete(&entity.LivePhoto{}, p.ID).Error; err != nil {
		slog.Error("live: DeletePhoto failed", "user_id", userId, "id", photoId, "error", err)
		return err
	}
	return nil
}

// photosFor loads the photos of several shows; every show gets a non-nil
// slice. Tickets first: the show page puts them apart.
func (s *Service) photosFor(showIds []uint) (map[uint][]domain.LivePhoto, error) {
	out := make(map[uint][]domain.LivePhoto, len(showIds))
	for _, id := range showIds {
		out[id] = []domain.LivePhoto{}
	}
	if len(showIds) == 0 {
		return out, nil
	}
	var rows []entity.LivePhoto
	if err := s.db.Preload("Image").Where("show_id IN ?", showIds).
		Order("is_ticket DESC, ord, id").Find(&rows).Error; err != nil {
		slog.Error("live: photosFor failed", "error", err)
		return nil, err
	}
	for i := range rows {
		out[rows[i].ShowID] = append(out[rows[i].ShowID], photoDto(&rows[i]))
	}
	return out, nil
}
