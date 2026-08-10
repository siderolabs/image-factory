// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package schedule computes when a daily, wall-clock-anchored task runs next.
package schedule

import (
	"fmt"
	"time"
)

// ParseTimeOfDay parses a "HH:MM" time of day into an offset from midnight. An
// empty string means "no schedule" and yields a zero offset.
func ParseTimeOfDay(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}

	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("invalid time of day %q, expected \"HH:MM\": %w", s, err)
	}

	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute, nil
}

// UntilNext returns the delay from now until the next occurrence of the
// offset-from-midnight timeOfDay, in now's location.
//
// The delay is always positive: at exactly the scheduled time the next
// occurrence is tomorrow, so a caller looping on it cannot spin.
func UntilNext(now time.Time, timeOfDay time.Duration) time.Duration {
	// build the target from the calendar date rather than by rounding the
	// timestamp, so a DST shift moves the schedule with the wall clock.
	next := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(timeOfDay)
	if !next.After(now) {
		next = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()).Add(timeOfDay)
	}

	return next.Sub(now)
}
