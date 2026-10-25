package live

import (
	"os"
	"path"
	"testing"

	"github.com/sbondCo/Watcharr/config"
	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/image"
)

// The nightly image cleanup keeps only images some foreign key points to
// (upstream's, since v4.3.0): live posters and photos must be among them,
// soft-deleted shows included, while an orphan still goes.
func TestCleanupImagesKeepsLiveImages(t *testing.T) {
	s := newTestService(t)

	img := func(name string) entity.Image {
		rel := path.Join("img", "live", name+".jpg")
		full := path.Join(config.DataPath, rel)
		if err := os.MkdirAll(path.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("jpg"), 0o644); err != nil {
			t.Fatal(err)
		}
		i := entity.Image{Hash: name, Path: rel}
		if err := s.db.Create(&i).Error; err != nil {
			t.Fatal(err)
		}
		return i
	}
	poster, deletedPoster, photo, orphan := img("poster"), img("deleted-poster"), img("photo"), img("orphan")

	show := mustCreate(t, s, 1, "Kept", "2026-11-05T20:00:00Z")
	if err := s.db.Model(&entity.LiveShow{}).Where("id = ?", show.ID).Update("poster_id", poster.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&entity.LivePhoto{ShowID: show.ID, ImageID: photo.ID}).Error; err != nil {
		t.Fatal(err)
	}
	gone := mustCreate(t, s, 1, "Deleted", "2026-11-06T20:00:00Z")
	if err := s.db.Model(&entity.LiveShow{}).Where("id = ?", gone.ID).Update("poster_id", deletedPoster.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Delete(&entity.LiveShow{}, gone.ID).Error; err != nil {
		t.Fatal(err)
	}

	image.CleanupImages(s.db)

	for _, c := range []struct {
		img  entity.Image
		kept bool
	}{{poster, true}, {deletedPoster, true}, {photo, true}, {orphan, false}} {
		var n int64
		s.db.Model(&entity.Image{}).Where("id = ?", c.img.ID).Count(&n)
		_, statErr := os.Stat(path.Join(config.DataPath, c.img.Path))
		if (n == 1) != c.kept || (statErr == nil) != c.kept {
			t.Errorf("%s: row kept=%v, file kept=%v, want %v", c.img.Hash, n == 1, statErr == nil, c.kept)
		}
	}
}
