// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transport

import (
	"io"
	"net/http"
	"strings"

	"github.com/felixge/httpsnoop"
)

// ResponseState records the final response status and pinned cache policy.
type ResponseState struct {
	pinnedCacheControl string
	status             int
}

// NewResponseState creates empty response observation state.
func NewResponseState() *ResponseState {
	return &ResponseState{}
}

// Status returns the first final status committed by the handler.
func (state *ResponseState) Status() int {
	return state.status
}

// PinCacheControl preserves the response's current Cache-Control value at commit time.
func (state *ResponseState) PinCacheControl(writer http.ResponseWriter) {
	state.pinnedCacheControl = strings.Join(writer.Header().Values("Cache-Control"), ", ")
}

// ApplyCacheControlPin restores a pinned Cache-Control value before the response commits.
func (state *ResponseState) ApplyCacheControlPin(writer http.ResponseWriter) {
	if state.pinnedCacheControl != "" {
		writer.Header().Set("Cache-Control", state.pinnedCacheControl)
	}
}

// ObserveResponse wraps writer while preserving its exact optional interface set.
func ObserveResponse(writer http.ResponseWriter, state *ResponseState) http.ResponseWriter {
	commit := func(status int) {
		if state.status != 0 {
			return
		}

		state.status = status
		state.ApplyCacheControlPin(writer)
	}

	return httpsnoop.Wrap(writer, httpsnoop.Hooks{
		WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
			return func(status int) {
				if status >= http.StatusOK {
					commit(status)
				}

				next(status)
			}
		},
		Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
			return func(body []byte) (int, error) {
				commit(http.StatusOK)

				return next(body)
			}
		},
		ReadFrom: func(next httpsnoop.ReadFromFunc) httpsnoop.ReadFromFunc {
			return func(source io.Reader) (int64, error) {
				commit(http.StatusOK)

				return next(source)
			}
		},
		Flush: func(next httpsnoop.FlushFunc) httpsnoop.FlushFunc {
			return func() {
				commit(http.StatusOK)
				next()
			}
		},
	})
}
