package discover

import (
	"testing"
	"time"
)

// Cinema weeks turn over on Wednesday, so "next week" is the coming Wednesday
// through the Tuesday after it — and a Wednesday is still part of this week.
func TestNextProgrammeWeek(t *testing.T) {
	for _, tc := range []struct {
		name            string
		now, start, end string
	}{
		{"a Saturday", "2026-08-15", "2026-08-19", "2026-08-25"},
		{"the Wednesday itself is still this week", "2026-08-19", "2026-08-26", "2026-09-01"},
		{"the Tuesday before turnover", "2026-08-18", "2026-08-19", "2026-08-25"},
		{"the Thursday after turnover", "2026-08-20", "2026-08-26", "2026-09-01"},
		{"across a month boundary", "2026-08-29", "2026-09-02", "2026-09-08"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now, err := time.Parse(time.DateOnly, tc.now)
			if err != nil {
				t.Fatal(err)
			}
			start, end := nextProgrammeWeek(now)
			if got := start.Format(time.DateOnly); got != tc.start {
				t.Errorf("start = %s, want %s", got, tc.start)
			}
			if got := end.Format(time.DateOnly); got != tc.end {
				t.Errorf("end = %s, want %s", got, tc.end)
			}
			if start.Weekday() != time.Wednesday {
				t.Errorf("start fell on %s, want Wednesday", start.Weekday())
			}
		})
	}
}
