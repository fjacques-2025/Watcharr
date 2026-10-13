package live

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/image"
	"gorm.io/gorm"
)

// Posters go under img/live/, next to the other cached images.
const posterSubPath = "live"

func posterError(reason string) error {
	return invalid("could not use that poster image: %s", reason)
}

// downloadPoster fetches the poster with the safe client. Done before the
// transaction: a 10 s download must not hold the SQLite write lock.
//
// Not image.Saver.DownloadAndInsertFromUrl: it uses the default client, which
// would happily fetch from the LAN.
func (s *Service) downloadPoster(raw string) ([]byte, error) {
	u, err := s.fetch.parseURL(raw)
	if err != nil {
		return nil, posterError(err.Error())
	}
	res, err := s.fetch.get(context.Background(), u, maxImageBytes, false)
	if err != nil {
		slog.Warn("live: poster download failed", "url", u.Redacted(), "reason", err)
		var verr *ValidationError
		switch {
		case errors.Is(err, errBlockedAddress):
			return nil, posterError("address not allowed")
		case errors.As(err, &verr):
			return nil, posterError(verr.Msg)
		case errors.Is(err, errTooBig):
			return nil, posterError("file is too big")
		default:
			return nil, posterError("download failed")
		}
	}
	return res.Body, nil
}

// Messages of image.Saver's own checks: a bad image is the user's link, a
// 400. Anything else from the saver (disk, database) stays a 500.
var saverRejections = map[string]string{
	"file is too big":                 "file is too big",
	"invalid or bad image":            "not a JPEG, PNG or GIF image",
	"unsupported image type":          "not a JPEG, PNG or GIF image",
	"invalid or corrupt image":        "the image is corrupt",
	"dimensions too large":            "the image is too large",
	"i can't handle all those pixels": "the image has too many pixels",
}

// attachPoster validates, re-encodes and stores the poster through the shared
// image saver (format, size and pixel limits, dedup by hash), then points the
// show at it.
func attachPoster(tx *gorm.DB, e *entity.LiveShow, b []byte) error {
	img, err := image.NewSaver(tx, posterSubPath, image.ValidateOptions{}).
		DownloadAndInsert(bytes.NewReader(b))
	if err != nil {
		if reason, ok := saverRejections[err.Error()]; ok {
			return posterError(reason)
		}
		return fmt.Errorf("storing poster: %w", err)
	}
	e.PosterID = &img.ID
	e.Poster = &img
	return nil
}
