// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package version

import (
	"sync"

	"github.com/siderolabs/image-factory/pkg/constants"
)

// ServerString is the Server header string including enterprise info if enabled.
var ServerString = sync.OnceValue(func() string {
	server := constants.ImageFactoryName
	server += " " + Tag

	return server
})
