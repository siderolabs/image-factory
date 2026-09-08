// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/asset"
	httpapi "github.com/siderolabs/image-factory/internal/frontend/http/api"
)

func TestPXEHandlerRejectsMissingExternalURLBeforeResolution(t *testing.T) {
	t.Parallel()

	for _, authEnabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "public", true: "authenticated"}[authEnabled], func(t *testing.T) {
			t.Parallel()

			service := &pxeService{}
			handler := httpapi.NewPXEHandler(service, httpapi.PXEHandlerOptions{AuthEnabled: authEnabled})

			err := handler.Serve(
				t.Context(),
				httptest.NewRecorder(),
				httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/pxe/id/v1/metal-amd64", nil),
				nil,
			)
			require.ErrorIs(t, err, httpapi.ErrExternalPXEURLRequired)
			require.Zero(t, service.calls)
		})
	}
}

type pxeService struct {
	calls int
}

func (service *pxeService) ResolvePXE(context.Context, asset.PXERequest) (asset.ResolvedPXE, error) {
	service.calls++

	return asset.ResolvedPXE{}, errors.New("unexpected PXE resolution")
}
