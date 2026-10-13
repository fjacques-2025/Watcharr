package live

import (
	"log/slog"
	"net/url"
	"strings"

	"github.com/sbondCo/Watcharr/database/entity"
)

// siteHost is the host a link is compared on to tell "the same site": lower
// case, without port nor a leading "www.". Empty when the link has no host.
func siteHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	h := strings.ToLower(u.Hostname())
	return strings.TrimPrefix(h, "www.")
}

// rememberedVenue is the venue the user typed for their most recently updated
// show from the same site as pageURL. Some theatres (La Criée) never name
// themselves on their pages: this spares retyping it for every show.
//
// Hosts are compared in Go, not with SQL LIKE: the user's shows are few, and
// the comparison must ignore case and "www.". A failed lookup only loses the
// suggestion, never the prefill.
func (s *Service) rememberedVenue(userId uint, pageURL *url.URL) string {
	host := siteHost(pageURL.String())
	if host == "" {
		return ""
	}
	var rows []entity.LiveShow
	// Soft-deleted rows are excluded by GORM. Most recent first, id breaking
	// ties so the pick is stable.
	res := s.db.Model(&entity.LiveShow{}).
		Select("url", "venue", "updated_at", "id").
		Where("user_id = ? AND url <> '' AND venue <> ''", userId).
		Order("updated_at DESC, id DESC").
		Find(&rows)
	if res.Error != nil {
		slog.Error("live: rememberedVenue failed", "user_id", userId, "error", res.Error)
		return ""
	}
	for _, r := range rows {
		if siteHost(r.URL) == host {
			if v := cleanText(r.Venue); v != "" {
				return v
			}
		}
	}
	return ""
}
