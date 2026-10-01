// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// robotsBrowserLogin lets crawlers index the sign-in landing page served at exactly "/"
// (the "$" anchor keeps query variants and every other path out), plus the assets it
// needs to render. The longer Allow rules win over "Disallow: /".
const robotsBrowserLogin = `User-agent: *
Allow: /$
Allow: /css/
Allow: /favicons/
Disallow: /
`

// robotsDisallowAll is served when authentication is enabled without browser login: the
// UI then answers with a Basic challenge, so there is nothing worth indexing.
const robotsDisallowAll = `User-agent: *
Disallow: /
`

// robotsText returns the robots.txt body for the deployment, or nil when authentication is
// disabled and the route is not registered at all.
func robotsText(authEnabled, browserLoginEnabled bool) []byte {
	switch {
	case !authEnabled:
		return nil
	case browserLoginEnabled:
		return []byte(robotsBrowserLogin)
	default:
		return []byte(robotsDisallowAll)
	}
}

func (f *Frontend) handleRobots(_ context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	if _, err := w.Write(f.robots); err != nil {
		return fmt.Errorf("write robots.txt: %w", err)
	}

	return nil
}
