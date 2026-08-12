// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"io"
	"net/http"
	"strings"

	"github.com/felixge/httpsnoop"
)

// responseState records what a handler actually sent, so the log and the audit record can
// report the client's status instead of inferring it from the returned error.
type responseState struct {
	pinnedCacheControl string
	status             int
}

// pinCacheControl fixes the current Cache-Control so a later handler cannot weaken a no-store.
func (s *responseState) pinCacheControl(w http.ResponseWriter) {
	s.pinnedCacheControl = strings.Join(w.Header().Values("Cache-Control"), ", ")
}

// applyCacheControlPin restores the pinned value over whatever a later handler set.
func (s *responseState) applyCacheControlPin(w http.ResponseWriter) {
	if s.pinnedCacheControl != "" {
		w.Header().Set("Cache-Control", s.pinnedCacheControl)
	}
}

// wrapResponseWriter wraps w to report into state, keeping w's exact interface set.
func wrapResponseWriter(w http.ResponseWriter, state *responseState) http.ResponseWriter {
	// Only the first response counts, which is the one net/http would keep.
	commit := func(status int) {
		if state.status != 0 {
			return
		}

		state.status = status

		state.applyCacheControlPin(w)
	}

	return httpsnoop.Wrap(w, httpsnoop.Hooks{
		WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
			return func(status int) {
				// An informational 1xx precedes the real response, which is still to come.
				if status >= 200 {
					commit(status)
				}

				next(status)
			}
		},

		// Also covers WriteString, which httpsnoop routes here.
		Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
			return func(b []byte) (int, error) {
				commit(http.StatusOK)

				return next(b)
			}
		},

		ReadFrom: func(next httpsnoop.ReadFromFunc) httpsnoop.ReadFromFunc {
			return func(src io.Reader) (int64, error) {
				commit(http.StatusOK)

				return next(src)
			}
		},

		// A flush commits the header, so settle the status and the pin first.
		Flush: func(next httpsnoop.FlushFunc) httpsnoop.FlushFunc {
			return func() {
				commit(http.StatusOK)
				next()
			}
		},
	})
}
