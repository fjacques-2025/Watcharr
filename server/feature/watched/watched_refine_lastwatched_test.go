package watched

import (
	"testing"
	"time"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"github.com/sbondCo/Watcharr/internal/testutil"
)

// "Last Watched" orders by the latest play OR finished episode/season, where
// "Last Finished" only sees plays.
func TestRefineSortLastWatched(t *testing.T) {
	testutil.SetupLogging()
	db := testutil.SetupDB(t)

	const uid = 1
	day := func(s string) time.Time {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatal(err)
		}
		return d.UTC()
	}
	now := time.Now().UTC()

	watched := map[string]uint{}
	for _, name := range []string{"finishedInJanuary", "newSeasonEpisode", "importedEpisodes", "seasonByHand", "nothing", "otherUser"} {
		w := entity.Watched{UserID: uid, Status: entity.WATCHING}
		if name == "otherUser" {
			w.UserID = 2
		}
		if err := db.Create(&w).Error; err != nil {
			t.Fatal(err)
		}
		watched[name] = w.ID
	}
	actBy := func(by entity.ActivityCreatedBy, name string, typ entity.ActivityType, data string, play bool, created time.Time, custom *time.Time) {
		a := entity.Activity{UserID: uid, WatchedID: watched[name], Type: typ, Data: data,
			CountAsPlay: play, CustomDate: custom, CreatedBy: by}
		a.CreatedAt = created
		if name == "otherUser" {
			a.UserID = 2
		}
		if err := db.Create(&a).Error; err != nil {
			t.Fatal(err)
		}
	}
	act := func(name string, typ entity.ActivityType, data string, play bool, created time.Time, custom *time.Time) {
		actBy(0, name, typ, data, play, created, custom)
	}
	jan, jun := day("2026-01-01"), day("2025-06-01")
	imported := day("2024-03-01")

	// Finished (a play, plain-text data) in January.
	act("finishedInJanuary", entity.STATUS_CHANGED, "FINISHED", true, jan, nil)
	// Played in June 2025, then an episode of a new season ticked yesterday;
	// the automatic show-status change it caused is not FINISHED.
	act("newSeasonEpisode", entity.STATUS_CHANGED, "FINISHED", true, jun, nil)
	act("newSeasonEpisode", entity.EPISODE_STATUS_CHANGED, `{"season":4,"episode":2,"status":"FINISHED"}`, false, now.Add(-24*time.Hour), nil)
	act("newSeasonEpisode", entity.STATUS_CHANGED, "WATCHING", false, now.Add(-24*time.Hour), nil)
	// Imported episodes: created at import time, real date in custom_date.
	// An episode merely set to WATCHING later doesn't count.
	act("importedEpisodes", entity.EPISODE_ADDED, `{"season":1,"episode":1,"status":"FINISHED","rating":0}`, false, now.Add(-48*time.Hour), &imported)
	act("importedEpisodes", entity.EPISODE_STATUS_CHANGED, `{"season":1,"episode":2,"status":"WATCHING"}`, false, now, nil)
	// The season Watcharr finished by itself during the import: dated by the
	// import, it must not lift the show either.
	actBy(entity.ActivityCreatedByWatcharr, "importedEpisodes", entity.SEASON_STATUS_CHANGED,
		`{"season":1,"status":"FINISHED"}`, false, now.Add(-48*time.Hour), nil)
	// A season the user set to FINISHED by hand does count.
	act("seasonByHand", entity.SEASON_STATUS_CHANGED, `{"season":1,"status":"FINISHED"}`, false, day("2025-01-01"), nil)
	// Another user's activity on their own entry must not leak in.
	act("otherUser", entity.STATUS_CHANGED, "FINISHED", true, now, nil)

	order := func(sort domain.WatchedSort) []string {
		var ws []entity.Watched
		err := db.Model(&entity.Watched{}).
			Where("watcheds.user_id = ?", uid).
			Scopes(watchedRefineSort(domain.WatchedGetPageRequest{Sort: sort, SortDir: domain.WatchedSortDirDesc}, uid)).
			Find(&ws).Error
		if err != nil {
			t.Fatalf("%s: %v", sort, err)
		}
		names := map[uint]string{}
		for n, id := range watched {
			names[id] = n
		}
		var out []string
		for _, w := range ws {
			out = append(out, names[w.ID])
		}
		return out
	}

	want := []string{"newSeasonEpisode", "finishedInJanuary", "seasonByHand", "importedEpisodes", "nothing"}
	if got := order(domain.WatchedSortLastWatched); !equal(got, want) {
		t.Errorf("Last Watched = %v, want %v", got, want)
	}
	// Last Finished unchanged: the episode doesn't lift the show.
	if got := order(domain.WatchedSortLastFinished); len(got) < 2 || got[0] != "finishedInJanuary" || got[1] != "newSeasonEpisode" {
		t.Errorf("Last Finished = %v, want finishedInJanuary then newSeasonEpisode first", got)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
