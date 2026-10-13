package live

import (
	"errors"
	"log/slog"
	"sort"
	"strings"
	"unicode"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
)

// ErrPersonNotFound covers another user's artist too, like ErrNotFound.
var ErrPersonNotFound = errors.New("artist not found")

const maxSearchResults = 20

// nameKey is a person's identity: case and spacing do not make a new artist,
// accents do (see entity.LivePerson).
func nameKey(name string) string {
	return strings.ToLower(collapseSpaces(name))
}

// searchFold is looser than nameKey: typing "elise" must find "Élise".
func searchFold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(nameKey(s)) {
		switch {
		case unicode.Is(unicode.Mn, r):
			// Combining accent left by NFD.
		case r == 'œ':
			b.WriteString("oe")
		case r == 'æ':
			b.WriteString("ae")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// resolvePerson finds or creates the artist a credit line names. A TMDB id
// wins over the name; a name already linked to another TMDB person is a
// homonym the user must sort out, never merged silently.
func resolvePerson(tx *gorm.DB, userId uint, c creditFields) (*entity.LivePerson, error) {
	var p entity.LivePerson
	if c.TmdbID != nil {
		res := tx.Where("user_id = ? AND tmdb_person_id = ?", userId, *c.TmdbID).Limit(1).Find(&p)
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected > 0 {
			return &p, nil
		}
	}
	res := tx.Where("user_id = ? AND name_key = ?", userId, nameKey(c.Name)).Limit(1).Find(&p)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		p = entity.LivePerson{UserID: userId, Name: c.Name, NameKey: nameKey(c.Name), TmdbPersonID: c.TmdbID}
		if err := tx.Create(&p).Error; err != nil {
			return nil, err
		}
		return &p, nil
	}
	if c.TmdbID != nil {
		if p.TmdbPersonID != nil {
			// Same id was looked up above, so this one is different.
			return nil, invalid("another artist named %q is already linked to a different TMDB person", c.Name)
		}
		// The stored name is kept: it is the user's spelling, a show save
		// only links the artist.
		if err := tx.Model(&p).Update("tmdb_person_id", *c.TmdbID).Error; err != nil {
			return nil, err
		}
		p.TmdbPersonID = c.TmdbID
	}
	return &p, nil
}

// replaceCredits writes a show's whole credit list, in the given order.
func replaceCredits(tx *gorm.DB, userId, showId uint, cs []creditFields) error {
	if err := tx.Where("show_id = ?", showId).Delete(&entity.LiveCredit{}).Error; err != nil {
		return err
	}
	type key struct {
		person uint
		role   string
	}
	seen := map[key]bool{}
	rows := make([]entity.LiveCredit, 0, len(cs))
	for _, c := range cs {
		p, err := resolvePerson(tx, userId, c)
		if err != nil {
			return err
		}
		// Two lines naming the same artist in the same role add nothing.
		k := key{p.ID, c.Role}
		if seen[k] {
			continue
		}
		seen[k] = true
		rows = append(rows, entity.LiveCredit{ShowID: showId, PersonID: p.ID, Role: c.Role, Ord: len(rows)})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

type creditRow struct {
	ShowID       uint
	PersonID     uint
	Role         string
	Name         string
	TmdbPersonID *int
}

// creditsFor loads the credits of several shows in one query. Every given
// show gets a non-nil slice, so the JSON is `[]` rather than `null`.
func (s *Service) creditsFor(showIds []uint) (map[uint][]domain.LiveCredit, error) {
	out := make(map[uint][]domain.LiveCredit, len(showIds))
	for _, id := range showIds {
		out[id] = []domain.LiveCredit{}
	}
	if len(showIds) == 0 {
		return out, nil
	}
	var rows []creditRow
	err := s.db.Table("live_credits AS c").
		Select("c.show_id, c.person_id, c.role, p.name, p.tmdb_person_id").
		Joins("JOIN live_people AS p ON p.id = c.person_id AND p.deleted_at IS NULL").
		Where("c.show_id IN ?", showIds).
		Order("c.show_id, c.ord, c.id").
		Scan(&rows).Error
	if err != nil {
		slog.Error("live: creditsFor failed", "error", err)
		return nil, err
	}
	for _, r := range rows {
		out[r.ShowID] = append(out[r.ShowID], domain.LiveCredit{
			PersonID: r.PersonID, Name: r.Name, Role: r.Role, TmdbPersonID: r.TmdbPersonID,
		})
	}
	return out, nil
}

// toDtos converts shows, loading their credits.
func (s *Service) toDtos(rows []entity.LiveShow) ([]domain.LiveShow, error) {
	ids := make([]uint, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	credits, err := s.creditsFor(ids)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]domain.LiveShow, 0, len(rows))
	for i := range rows {
		out = append(out, s.toDto(&rows[i], now, credits[rows[i].ID]))
	}
	return out, nil
}

func (s *Service) toDtoWithCredits(e *entity.LiveShow) (domain.LiveShow, error) {
	shows, err := s.toDtos([]entity.LiveShow{*e})
	if err != nil {
		return domain.LiveShow{}, err
	}
	return shows[0], nil
}

// People lists the user's artists that appear in at least one show still in
// the notebook. q, if not blank, filters on the name ignoring case and
// accents.
func (s *Service) People(userId uint, q string) ([]domain.LivePersonSummary, error) {
	var rows []domain.LivePersonSummary
	// Inner joins: an artist whose shows were all deleted drops out.
	err := s.db.Table("live_people AS p").
		Select("p.id, p.name, p.tmdb_person_id, COUNT(DISTINCT c.show_id) AS show_count").
		Joins("JOIN live_credits AS c ON c.person_id = p.id").
		Joins("JOIN live_shows AS s ON s.id = c.show_id AND s.user_id = p.user_id AND s.deleted_at IS NULL").
		Where("p.user_id = ? AND p.deleted_at IS NULL", userId).
		Group("p.id").
		Scan(&rows).Error
	if err != nil {
		slog.Error("live: People failed", "user_id", userId, "error", err)
		return nil, err
	}
	// Roles for the list's chips and its role filter: a second, small query
	// rather than a GROUP_CONCAT, whose separator a role could contain.
	var held []struct {
		PersonID uint
		Role     string
	}
	err = s.db.Table("live_credits AS c").
		Select("DISTINCT c.person_id, c.role").
		Joins("JOIN live_shows AS s ON s.id = c.show_id AND s.user_id = ? AND s.deleted_at IS NULL", userId).
		Scan(&held).Error
	if err != nil {
		slog.Error("live: People roles failed", "user_id", userId, "error", err)
		return nil, err
	}
	roles := map[uint][]string{}
	for _, h := range held {
		roles[h.PersonID] = append(roles[h.PersonID], h.Role)
	}
	for i := range rows {
		r := roles[rows[i].ID]
		if r == nil {
			r = []string{}
		}
		sort.Strings(r)
		rows[i].Roles = r
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
	out := make([]domain.LivePersonSummary, 0, len(rows))
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

func (s *Service) findPerson(userId, id uint) (*entity.LivePerson, error) {
	var p entity.LivePerson
	res := s.db.Where("id = ? AND user_id = ?", id, userId).Limit(1).Find(&p)
	if res.Error != nil {
		slog.Error("live: findPerson failed", "user_id", userId, "id", id, "error", res.Error)
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrPersonNotFound
	}
	return &p, nil
}

func (s *Service) Person(userId, id uint) (domain.LivePersonDetail, error) {
	p, err := s.findPerson(userId, id)
	if err != nil {
		return domain.LivePersonDetail{}, err
	}
	return s.personDetail(userId, p)
}

// personDetail groups the artist's shows by role: biggest role first, most
// recent show first within a role.
func (s *Service) personDetail(userId uint, p *entity.LivePerson) (domain.LivePersonDetail, error) {
	d := domain.LivePersonDetail{ID: p.ID, Name: p.Name, TmdbPersonID: p.TmdbPersonID, Roles: []domain.LivePersonRole{}}

	var credits []entity.LiveCredit
	if err := s.db.Where("person_id = ?", p.ID).Find(&credits).Error; err != nil {
		slog.Error("live: personDetail credits failed", "id", p.ID, "error", err)
		return d, err
	}
	if len(credits) == 0 {
		return d, nil
	}
	showIds := make([]uint, 0, len(credits))
	for _, c := range credits {
		showIds = append(showIds, c.ShowID)
	}
	var rows []entity.LiveShow
	// The default scope drops deleted shows; user_id is belt and braces.
	if err := s.db.Preload("Poster").Where("id IN ? AND user_id = ?", showIds, userId).
		Order("date DESC, id DESC").Find(&rows).Error; err != nil {
		slog.Error("live: personDetail shows failed", "id", p.ID, "error", err)
		return d, err
	}
	shows, err := s.toDtos(rows)
	if err != nil {
		return d, err
	}

	rolesOf := map[uint]map[string]bool{}
	for _, c := range credits {
		if rolesOf[c.ShowID] == nil {
			rolesOf[c.ShowID] = map[string]bool{}
		}
		rolesOf[c.ShowID][c.Role] = true
	}
	byRole := map[string][]domain.LiveShow{}
	// shows is date-ordered, so each role's list comes out ordered too.
	for _, sh := range shows {
		for role := range rolesOf[sh.ID] {
			byRole[role] = append(byRole[role], sh)
		}
	}
	for role, list := range byRole {
		d.Roles = append(d.Roles, domain.LivePersonRole{Role: role, Shows: list})
	}
	sort.Slice(d.Roles, func(i, j int) bool {
		a, b := d.Roles[i], d.Roles[j]
		if len(a.Shows) != len(b.Shows) {
			return len(a.Shows) > len(b.Shows)
		}
		if la, lb := strings.ToLower(a.Role), strings.ToLower(b.Role); la != lb {
			return la < lb
		}
		return a.Role < b.Role
	})
	return d, nil
}

// PersonByTmdb is the "On stage" section of a TMDB person page: nil when the
// user never saw that person on stage (or deleted those shows).
func (s *Service) PersonByTmdb(userId uint, tmdbId int) (*domain.LivePersonDetail, error) {
	var p entity.LivePerson
	res := s.db.Where("user_id = ? AND tmdb_person_id = ?", userId, tmdbId).Limit(1).Find(&p)
	if res.Error != nil {
		slog.Error("live: PersonByTmdb failed", "user_id", userId, "tmdb_id", tmdbId, "error", res.Error)
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	d, err := s.personDetail(userId, &p)
	if err != nil {
		return nil, err
	}
	if len(d.Roles) == 0 {
		return nil, nil
	}
	return &d, nil
}

var (
	errNameTaken = invalid("an artist with that name already exists")
	errTmdbTaken = invalid("another artist is already linked to that TMDB person")
)

// UpdatePerson renames an artist and links or unlinks TMDB. Merging two
// artists is not offered: a rename onto an existing name is refused.
func (s *Service) UpdatePerson(userId, id uint, r domain.LivePersonUpdateRequest) (domain.LivePerson, error) {
	name, err := validPersonName(r.Name)
	if err != nil {
		return domain.LivePerson{}, err
	}
	if err := validTmdbID(r.TmdbPersonID); err != nil {
		return domain.LivePerson{}, err
	}
	p, err := s.findPerson(userId, id)
	if err != nil {
		return domain.LivePerson{}, err
	}
	key := nameKey(name)
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&entity.LivePerson{}).
			Where("user_id = ? AND name_key = ? AND id <> ?", userId, key, id).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return errNameTaken
		}
		if r.TmdbPersonID != nil {
			if err := tx.Model(&entity.LivePerson{}).
				Where("user_id = ? AND tmdb_person_id = ? AND id <> ?", userId, *r.TmdbPersonID, id).
				Count(&n).Error; err != nil {
				return err
			}
			if n > 0 {
				return errTmdbTaken
			}
		}
		// Map, not struct: a nil TMDB id must be written to unlink.
		return tx.Model(p).Updates(map[string]any{
			"name": name, "name_key": key, "tmdb_person_id": r.TmdbPersonID,
		}).Error
	})
	if err != nil {
		// Lost a race with a concurrent save: same answer as the check.
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.LivePerson{}, errNameTaken
		}
		var verr *ValidationError
		if !errors.As(err, &verr) {
			slog.Error("live: UpdatePerson failed", "user_id", userId, "id", id, "error", err)
		}
		return domain.LivePerson{}, err
	}
	return domain.LivePerson{ID: p.ID, Name: name, TmdbPersonID: r.TmdbPersonID}, nil
}
