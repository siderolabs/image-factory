// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package schedule_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/schedule"
)

func TestParseTimeOfDay(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		in       string
		expected time.Duration
		err      bool
	}{
		{in: "", expected: 0},
		{in: "00:00", expected: 0},
		{in: "02:00", expected: 2 * time.Hour},
		{in: "07:30", expected: 7*time.Hour + 30*time.Minute},
		{in: "23:59", expected: 23*time.Hour + 59*time.Minute},
		{in: "2am", err: true},
		{in: "24:00", err: true},
		{in: "02:00:00", err: true},
	} {
		t.Run(test.in, func(t *testing.T) {
			t.Parallel()

			actual, err := schedule.ParseTimeOfDay(test.in)

			if test.err {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestUntilNext(t *testing.T) {
	t.Parallel()

	at := 2 * time.Hour

	for _, test := range []struct {
		now      time.Time
		name     string
		expected time.Duration
	}{
		{
			name:     "before",
			now:      time.Date(2026, 8, 10, 1, 30, 0, 0, time.UTC),
			expected: 30 * time.Minute,
		},
		{
			name:     "after",
			now:      time.Date(2026, 8, 10, 2, 30, 0, 0, time.UTC),
			expected: 23*time.Hour + 30*time.Minute,
		},
		{
			// exactly at the scheduled time: schedule the next day, never a zero
			// delay, which would spin the refresh loop.
			name:     "exact",
			now:      time.Date(2026, 8, 10, 2, 0, 0, 0, time.UTC),
			expected: 24 * time.Hour,
		},
		{
			name:     "month rollover",
			now:      time.Date(2026, 8, 31, 23, 0, 0, 0, time.UTC),
			expected: 3 * time.Hour,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, schedule.UntilNext(test.now, at))
		})
	}
}
