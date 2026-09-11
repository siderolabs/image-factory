// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package regtransport_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/regtransport"
)

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	missing := &transport.Error{StatusCode: http.StatusNotFound}
	require.True(t, regtransport.IsNotFound(missing))
	require.True(t, regtransport.IsNotFound(fmt.Errorf("lookup: %w", missing)))
	require.False(t, regtransport.IsNotFound(nil))
	require.False(t, regtransport.IsNotFound(errors.New("not found")))
	require.False(t, regtransport.IsNotFound(&transport.Error{StatusCode: http.StatusForbidden}))
}
