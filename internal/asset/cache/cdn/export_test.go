// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cdn

import "time"

// SetNowForTesting overrides the clock used to stamp tokens.
func (c *Cache) SetNowForTesting(now func() time.Time) {
	c.now = now
}
