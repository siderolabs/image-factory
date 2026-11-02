// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package spa

import (
	"net/http"
	"strings"
)

// setSecurityHeaders sets the content security policy and the other browser security headers of index.html.
//
// Vite puts the nonce on every script, style and stylesheet tag it writes into index.html (html.cspNonce in
// frontend/vite.config.ts), and on a csp-nonce meta tag that its client and preload helper take it from for the
// styles and stylesheets they add later. The dev server's index.html goes through here too, so the policy is the same
// in development.
func setSecurityHeaders(header http.Header, nonce string, connectSrc []string) {
	header.Set("Content-Security-Policy", strings.Join([]string{
		"default-src 'self'",
		"base-uri 'self'",
		// X-Frame-Options below does the same for the browsers that predate frame-ancestors.
		"frame-ancestors 'self'",
		"script-src 'self' 'nonce-" + nonce + "'",
		"style-src 'self' 'nonce-" + nonce + "'",
		// The stylesheets inline small images and fonts as data: URLs.
		"img-src 'self' data:",
		"font-src 'self' data:",
		// The dev server's hot reload websocket is same-origin as well, which 'self' covers.
		"connect-src " + strings.Join(connectSrc, " "),
	}, "; "))

	header.Set("X-Frame-Options", "SAMEORIGIN")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	header.Set("Permissions-Policy", strings.Join([]string{
		"accelerometer=()",
		"autoplay=()",
		"bluetooth=()",
		"camera=()",
		"clipboard-read=(self)",
		"clipboard-write=(self)",
		"cross-origin-isolated=()",
		"display-capture=()",
		"encrypted-media=()",
		"fullscreen=()",
		"geolocation=()",
		"gyroscope=()",
		"hid=()",
		"idle-detection=()",
		"magnetometer=()",
		"microphone=()",
		"midi=()",
		"payment=()",
		"picture-in-picture=()",
		"publickey-credentials-create=()",
		"publickey-credentials-get=()",
		"screen-wake-lock=()",
		"serial=()",
		"sync-xhr=()",
		"usb=()",
		"web-share=()",
		"window-management=()",
		"xr-spatial-tracking=()",
	}, ", "))
}
