package live

import (
	"testing"
	"time"

	"github.com/sbondCo/Watcharr/domain"
)

func TestShowState(t *testing.T) {
	now := time.Date(2026, 10, 13, 20, 30, 0, 0, time.UTC)
	tests := []struct {
		name   string
		missed bool
		date   time.Time
		want   domain.LiveShowState
	}{
		{"future", false, now.Add(24 * time.Hour), domain.LiveShowUpcoming},
		{"one nanosecond ahead", false, now.Add(time.Nanosecond), domain.LiveShowUpcoming},
		// The exact start time already counts as seen.
		{"exactly now", false, now, domain.LiveShowSeen},
		{"past", false, now.Add(-time.Hour), domain.LiveShowSeen},
		{"missed in the past", true, now.Add(-time.Hour), domain.LiveShowMissed},
		// Not having gone means nothing before the date: still upcoming.
		{"missed in the future", true, now.Add(time.Hour), domain.LiveShowUpcoming},
		{"missed one nanosecond ahead", true, now.Add(time.Nanosecond), domain.LiveShowUpcoming},
		{"missed exactly now", true, now, domain.LiveShowMissed},
		// Same instant in another offset is still the same instant.
		{"exactly now, other offset", false,
			now.In(time.FixedZone("CEST", 2*3600)), domain.LiveShowSeen},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShowState(tt.missed, tt.date, now); got != tt.want {
				t.Errorf("ShowState = %q, want %q", got, tt.want)
			}
		})
	}
}
