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
//
// "Didn't go" only means something once the date has passed, so a future
// show is upcoming whatever the flag says. The flag itself is kept as sent:
// if the date moves back into the past, the user's correction comes back.
func ShowState(missed bool, date, now time.Time) domain.LiveShowState {
	switch {
	case date.After(now):
		return domain.LiveShowUpcoming
	case missed:
		return domain.LiveShowMissed
	default:
		return domain.LiveShowSeen
	}
}
