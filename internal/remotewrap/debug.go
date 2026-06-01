// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package remotewrap

import (
	"io"
	"log"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// debugRegistry enables registry HTTP debugging; set via SetDebug from the
// registry.debug config option (or the IF_REGISTRY_DEBUG environment variable).
//
// When enabled it tracks every response body returned by the registry transport
// and logs any body that stays open for too long. Because go-containerregistry's
// pull limiter only releases its token when the blob/manifest body is Closed, a
// body that is never closed is exactly a leaked limiter token. The captured stack
// points at the code path that opened the body and forgot to close it.
var debugRegistry bool

// SetDebug toggles registry HTTP debugging. Call at startup, before any
// Puller/Pusher is constructed.
func SetDebug(enabled bool) {
	debugRegistry = enabled
}

// DebugEnabled reports whether registry HTTP debugging is enabled.
func DebugEnabled() bool {
	return debugRegistry
}

// RoundTripper returns the RoundTripper used for remote pull/push operations,
// wrapping the pooled transport with body tracking when debugging is enabled.
func RoundTripper() http.RoundTripper {
	return roundTripper()
}

var roundTripper = sync.OnceValue(func() http.RoundTripper {
	if !debugRegistry {
		return transport()
	}

	startDebugDumper()

	return &debugTransport{base: transport()}
})

type openBody struct {
	since  time.Time
	method string
	url    string
	stack  string
	id     uint64
}

var (
	openBodyID atomic.Uint64
	openBodies sync.Map // id -> *openBody
)

type debugTransport struct {
	base http.RoundTripper
}

func (d *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := d.base.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}

	id := openBodyID.Add(1)

	buf := make([]byte, 8<<10)
	buf = buf[:runtime.Stack(buf, false)]

	openBodies.Store(id, &openBody{
		id:     id,
		method: req.Method,
		url:    req.URL.String(),
		since:  time.Now(),
		stack:  string(buf),
	})

	resp.Body = &trackedReadCloser{ReadCloser: resp.Body, id: id}

	return resp, nil
}

type trackedReadCloser struct {
	io.ReadCloser

	id   uint64
	once sync.Once
}

func (t *trackedReadCloser) Close() error {
	err := t.ReadCloser.Close()

	t.once.Do(func() {
		openBodies.Delete(t.id)
	})

	return err
}

func startDebugDumper() {
	log.Printf("[registry-debug] enabled: tracking unclosed registry response bodies")

	go func() {
		for range time.Tick(10 * time.Second) {
			count := 0

			openBodies.Range(func(_, v any) bool {
				b := v.(*openBody) //nolint:forcetypeassert,errcheck

				count++

				if age := time.Since(b.since); age > 5*time.Second {
					log.Printf("[registry-debug] UNCLOSED body id=%d age=%s %s %s\n%s",
						b.id, age.Round(time.Second), b.method, b.url, b.stack)
				}

				return true
			})

			log.Printf("[registry-debug] open registry response bodies: %d", count)
		}
	}()
}
