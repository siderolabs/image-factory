// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"net/http"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
)

func TestRegisterRuntimeRouteRequiresOpenAPIOperation(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	registered := false
	registrator := func(string, httprouter.Handle) {
		registered = true
	}
	handle := func(http.ResponseWriter, *http.Request, httprouter.Params) {}

	err = registerRuntimeRoute(contract, http.MethodGet, registrator, "/future-route", handle)

	require.ErrorContains(t, err, "is not declared in OpenAPI")
	require.False(t, registered)
}

func TestRegisterRuntimeRouteAcceptsOpenAPIOperation(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	var registeredPath string

	registrator := func(path string, _ httprouter.Handle) {
		registeredPath = path
	}
	handle := func(http.ResponseWriter, *http.Request, httprouter.Params) {}

	err = registerRuntimeRoute(contract, http.MethodGet, registrator, "/css/*filepath", handle)

	require.NoError(t, err)
	require.Equal(t, "/css/*filepath", registeredPath)
}
