// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transport_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

func TestRegistrarPublishesWithoutExposingMutableRouter(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	pipeline, err := transport.NewPipeline(atomicDirectBuilder, atomicDirectBuilder, atomicDirectBuilder)
	require.NoError(t, err)

	registrar, err := transport.NewRegistrar(contract, pipeline)
	require.NoError(t, err)

	handler := registrar.Handler()
	_, mutable := handler.(interface {
		Handle(string, string, httprouter.Handle)
	})
	require.False(t, mutable)

	stop := make(chan struct{})

	var workers sync.WaitGroup

	for range 8 {
		workers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					handler.ServeHTTP(
						httptest.NewRecorder(),
						httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/versions", nil),
					)
				}
			}
		})
	}

	require.NoError(t, registrar.Register([]transport.Route{{
		Method:      http.MethodGet,
		Path:        "/versions",
		OperationID: "listVersions",
		Access:      transport.AccessPublic,
		Protocol:    transport.ProtocolAPI,
		Handler: func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
			return nil
		},
	}}))

	close(stop)
	workers.Wait()
}

func atomicDirectBuilder(route transport.Route) (httprouter.Handle, error) {
	return func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		if err := route.Handler(request.Context(), writer, request, params); err != nil {
			panic(err)
		}
	}, nil
}
