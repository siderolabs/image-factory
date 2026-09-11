// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transport_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

func TestPipelineSelectsProtocolPath(t *testing.T) {
	t.Parallel()

	handler := func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
		return nil
	}

	for _, test := range []struct {
		name     string
		want     string
		protocol transport.Protocol
	}{
		{name: "application", protocol: transport.ProtocolAPI, want: "application"},
		{name: "HTML", protocol: transport.ProtocolHTML, want: "application"},
		{name: "browser auth", protocol: transport.ProtocolBrowserAuth, want: "application"},
		{name: "operational", protocol: transport.ProtocolOperational, want: "application"},
		{name: "OCI", protocol: transport.ProtocolOCI, want: "oci"},
		{name: "static", protocol: transport.ProtocolStatic, want: "static"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			selected := ""
			builder := func(name string) transport.HandlerBuilder {
				return func(route transport.Route) (httprouter.Handle, error) {
					selected = name + ":" + accessName(route.Access)

					return testHandle(route), nil
				}
			}

			pipeline, err := transport.NewPipeline(builder("application"), builder("oci"), builder("static"))
			require.NoError(t, err)

			route := transport.Route{
				Method:      http.MethodGet,
				Path:        "/versions",
				OperationID: "listVersions",
				Access:      transport.AccessPublic,
				Protocol:    test.protocol,
				Handler:     handler,
			}
			if test.protocol == transport.ProtocolOCI {
				route.Path = "/v2/*path"
				route.OperationID = ""
				route.DispatchedOperationIDs = []string{"operation"}
				route.Access = transport.AccessImageDownload
			}

			if test.protocol == transport.ProtocolStatic {
				route.Path = "/css/*filepath"
				route.OperationID = "getCSSAsset"
			}

			_, err = pipeline.Handler(route)
			require.NoError(t, err)
			require.Equal(t, test.want+":"+accessName(route.Access), selected)
		})
	}
}

func TestPipelineRequiresEveryProtocolPath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		application transport.HandlerBuilder
		oci         transport.HandlerBuilder
		static      transport.HandlerBuilder
		wantErr     string
	}{
		{name: "application", oci: directBuilder, static: directBuilder, wantErr: "application pipeline is required"},
		{name: "OCI", application: directBuilder, static: directBuilder, wantErr: "OCI pipeline is required"},
		{name: "static", application: directBuilder, oci: directBuilder, wantErr: "static pipeline is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := transport.NewPipeline(test.application, test.oci, test.static)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func accessName(access transport.AccessPolicy) string {
	switch access {
	case transport.AccessPublic:
		return "public"
	case transport.AccessAuthenticated:
		return "authenticated"
	case transport.AccessImageDownload:
		return "image-download"
	default:
		return "unknown"
	}
}
