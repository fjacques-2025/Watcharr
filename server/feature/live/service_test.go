package live

import (
	"errors"
	"testing"
	"time"

	"github.com/sbondCo/Watcharr/config"
	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"github.com/sbondCo/Watcharr/internal/testutil"
)

var testNow = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

// newTestService runs the real migrations (testutil.SetupDB), so it also
// checks that LiveShow is registered for AutoMigrate.
//
// Its fetcher may reach 127.0.0.1 on any port, where httptest servers live:
// tests cannot use public sites. Images land in a temporary data dir.
func newTestService(t *testing.T) *Service {
	t.Helper()
	testutil.SetupLogging()
	s := NewService(testutil.SetupDB(t))
	s.now = func() time.Time { return testNow }
	s.fetch = buildFetcher(true, true)
	prev := config.DataPath
	config.DataPath = t.TempDir()
	t.Cleanup(func() { config.DataPath = prev })
	return s
}

func mustCreate(t *testing.T, s *Service, userId uint, title, date string) domain.LiveShow {
	t.Helper()
	show, err := s.Create(userId, domain.LiveShowRequest{Title: title, Date: date})
	if err != nil {
		t.Fatalf("Create(%q): %v", title, err)
	}
	return show
}

func TestTableName(t *testing.T) {
	s := newTestService(t)
	if !s.db.Migrator().HasTable("live_shows") {
		t.Fatal("table live_shows was not created")
	}
	if !s.db.Migrator().HasTable(&entity.LiveShow{}) {
		t.Fatal("LiveShow does not map to an existing table")
	}
}

func TestCreateAndGet(t *testing.T) {
	s := newTestService(t)
	created, err := s.Create(1, domain.LiveShowRequest{
		Title:  " Le Misanthrope ",
		Venue:  "La Criée",
		URL:    "https://theatre-lacriee.com/x",
		Date:   "2026-11-05T20:00:00+01:00",
		Missed: true, // must be ignored on create
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 || created.Title != "Le Misanthrope" {
		t.Errorf("unexpected created show: %+v", created)
	}
	if created.Missed || created.State != domain.LiveShowUpcoming {
		t.Errorf("new show should be upcoming and not missed, got missed=%v state=%q",
			created.Missed, created.State)
	}

	got, err := s.Get(1, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Date.Equal(time.Date(2026, 11, 5, 19, 0, 0, 0, time.UTC)) {
		t.Errorf("date = %v", got.Date)
	}
	if got.Venue != "La Criée" || got.URL != "https://theatre-lacriee.com/x" {
		t.Errorf("fields not persisted: %+v", got)
	}
}

func TestCreateRejectsInvalid(t *testing.T) {
	s := newTestService(t)
	_, err := s.Create(1, domain.LiveShowRequest{Title: "", Date: "2026-11-05T20:00:00Z"})
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("want *ValidationError, got %v", err)
	}
}

func TestListSortedAndStates(t *testing.T) {
	s := newTestService(t)
	// Mixed offsets on purpose: sorting must follow the instant, not the text.
	mustCreate(t, s, 1, "C", "2026-12-01T20:00:00+01:00")
	mustCreate(t, s, 1, "A", "2026-09-01T20:00:00+02:00")
	// 11:30 UTC, i.e. before testNow despite the later wall clock.
	mustCreate(t, s, 1, "B", "2026-10-13T13:30:00+02:00")
	mustCreate(t, s, 2, "other user", "2026-10-01T20:00:00Z")

	shows, err := s.List(1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var titles []string
	for _, sh := range shows {
		titles = append(titles, sh.Title)
	}
	if len(titles) != 3 || titles[0] != "A" || titles[1] != "B" || titles[2] != "C" {
		t.Fatalf("titles = %v, want [A B C]", titles)
	}
	if shows[0].State != domain.LiveShowSeen || shows[1].State != domain.LiveShowSeen ||
		shows[2].State != domain.LiveShowUpcoming {
		t.Errorf("states = %q %q %q", shows[0].State, shows[1].State, shows[2].State)
	}
}

func TestListEmptyIsNotNil(t *testing.T) {
	s := newTestService(t)
	shows, err := s.List(1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// The client gets `[]`, not `null`.
	if shows == nil || len(shows) != 0 {
		t.Fatalf("want an empty non-nil slice, got %#v", shows)
	}
}

func TestUpdateReplacesFields(t *testing.T) {
	s := newTestService(t)
	created, err := s.Create(1, domain.LiveShowRequest{
		Title: "Old", Venue: "Somewhere", Description: "desc", Date: "2026-11-05T20:00:00Z",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	upd, err := s.Update(1, created.ID, domain.LiveShowRequest{
		Title: "New", Date: "2026-10-01T20:00:00Z", Missed: true,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if upd.Title != "New" || upd.Venue != "" || upd.Description != "" ||
		!upd.Missed || upd.State != domain.LiveShowMissed {
		t.Errorf("not a full replace: %+v", upd)
	}

	// And un-marking missed must stick too (zero value written).
	if _, err := s.Update(1, created.ID, domain.LiveShowRequest{
		Title: "New", Date: "2026-10-01T20:00:00Z",
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := s.Get(1, created.ID)
	if got.Missed || got.State != domain.LiveShowSeen {
		t.Errorf("missed not cleared: %+v", got)
	}
}

func TestUpdateRejectsInvalid(t *testing.T) {
	s := newTestService(t)
	created := mustCreate(t, s, 1, "A", "2026-11-05T20:00:00Z")
	_, err := s.Update(1, created.ID, domain.LiveShowRequest{Title: "A", Date: "nope"})
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("want *ValidationError, got %v", err)
	}
}

func TestOtherUserCannotTouchShow(t *testing.T) {
	s := newTestService(t)
	a := mustCreate(t, s, 1, "A's show", "2026-11-05T20:00:00Z")

	if _, err := s.Get(2, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get by other user: want ErrNotFound, got %v", err)
	}
	if _, err := s.Update(2, a.ID, domain.LiveShowRequest{
		Title: "hijacked", Date: "2026-11-05T20:00:00Z",
	}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update by other user: want ErrNotFound, got %v", err)
	}
	if err := s.Delete(2, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete by other user: want ErrNotFound, got %v", err)
	}

	got, err := s.Get(1, a.ID)
	if err != nil || got.Title != "A's show" {
		t.Fatalf("owner's show was altered: %+v, %v", got, err)
	}
}

func TestNotFound(t *testing.T) {
	s := newTestService(t)
	if _, err := s.Get(1, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get: want ErrNotFound, got %v", err)
	}
	if _, err := s.Update(1, 999, domain.LiveShowRequest{
		Title: "x", Date: "2026-11-05T20:00:00Z",
	}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update: want ErrNotFound, got %v", err)
	}
	if err := s.Delete(1, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete: want ErrNotFound, got %v", err)
	}
}

func TestDeleteIsSoftAndHides(t *testing.T) {
	s := newTestService(t)
	a := mustCreate(t, s, 1, "A", "2026-11-05T20:00:00Z")
	mustCreate(t, s, 1, "B", "2026-11-06T20:00:00Z")

	if err := s.Delete(1, a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(1, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after delete: want ErrNotFound, got %v", err)
	}
	shows, _ := s.List(1)
	if len(shows) != 1 || shows[0].Title != "B" {
		t.Errorf("deleted show still listed: %+v", shows)
	}
	if err := s.Delete(1, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete: want ErrNotFound, got %v", err)
	}

	// Soft delete: the row is still there, just flagged.
	var count int64
	s.db.Unscoped().Model(&entity.LiveShow{}).Where("id = ?", a.ID).Count(&count)
	if count != 1 {
		t.Errorf("row was hard deleted (count=%d)", count)
	}
}
