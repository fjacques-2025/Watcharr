package live

import (
	"time"

	"github.com/sbondCo/Watcharr/domain"
)

// ShowState derives a show's state. `now` is a parameter so the boundary can
// be tested without a clock.
//
// A show is "seen" from the very moment it starts: the user annotates it
// afterwards, and correcting to "missed" is the exception, not the rule.
func ShowState(missed bool, date, now time.Time) domain.LiveShowState {
	switch {
	case missed:
		return domain.LiveShowMissed
	case date.After(now):
		return domain.LiveShowUpcoming
	default:
		return domain.LiveShowSeen
	}
}
