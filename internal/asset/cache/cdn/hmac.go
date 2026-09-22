// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cdn

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strconv"
	"time"
)

// DefaultHMACParam is the query parameter carrying the token.
const DefaultHMACParam = "verify"

// Token builds a timed-HMAC token over message at issue time ts.
//
// Format: "<unix-seconds>-<base64url(HMAC-SHA256(secret, message + unix-seconds))>", unpadded.
// The WAF rule validating it must pass flags 's' to select that alphabet.
func Token(secret []byte, message string, ts time.Time) string {
	stamp := strconv.FormatInt(ts.Unix(), 10)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(message))
	mac.Write([]byte(stamp))

	return stamp + "-" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// appendToken appends the token to u, leaving the existing query string byte-for-byte intact.
//
// Cloudflare parses http.request.uri positionally from the end — MAC, timestamp, then
// lengthOfSeparator bytes — and treats everything before that as the message. So the token
// must be last in the query string, and the message is the whole URI preceding it.
func appendToken(u *url.URL, secret []byte, param string, ts time.Time) {
	message := u.EscapedPath()
	if u.RawQuery != "" {
		message += "?" + u.RawQuery
	}

	// RawURLEncoding output needs no escaping, so the query is only ever appended to.
	token := param + "=" + Token(secret, message, ts)

	if u.RawQuery == "" {
		u.RawQuery = token
	} else {
		u.RawQuery += "&" + token
	}
}
