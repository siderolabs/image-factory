// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package authn_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/authn"
)

func TestPrincipalRoundTrip(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		credential authn.Credential
	}{
		{name: "provider", credential: authn.CredentialProvider},
		{name: "API token", credential: authn.CredentialAPIToken},
		{name: "image download token", credential: authn.CredentialImageDownloadToken},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			principal, err := authn.NewPrincipal("acme", test.credential)
			require.NoError(t, err)
			require.True(t, principal.Authenticated())
			require.Equal(t, "acme", principal.Username())
			require.Equal(t, test.credential, principal.Credential())

			stored, ok := authn.PrincipalFromContext(authn.ContextWithPrincipal(context.Background(), principal))
			require.True(t, ok)
			require.Equal(t, principal, stored)
		})
	}
}

func TestAnonymousContext(t *testing.T) {
	t.Parallel()

	principal, ok := authn.PrincipalFromContext(context.Background())
	require.False(t, ok)
	require.False(t, principal.Authenticated())
	require.Equal(t, authn.CredentialAnonymous, principal.Credential())
}

func TestPrincipalRejectsInvalidIdentity(t *testing.T) {
	t.Parallel()

	_, err := authn.NewPrincipal("", authn.CredentialProvider)
	require.ErrorContains(t, err, "username")

	_, err = authn.NewPrincipal("acme", authn.CredentialAnonymous)
	require.ErrorContains(t, err, "credential")

	_, err = authn.NewPrincipal("acme", authn.Credential(255))
	require.ErrorContains(t, err, "credential")
}
