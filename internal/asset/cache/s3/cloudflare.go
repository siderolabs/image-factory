// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package s3

import (
	"os"

	"github.com/minio/minio-go/v7/pkg/credentials"
)

// A EnvCloudflare retrieves credentials from the environment variables of the
// running process. EnvCloudflare credentials never expire.
//
// EnvCloudflare variables used:
//
// * Access Key ID:     CF_ACCESS_KEY.
// * Secret Access Key: CF_SECRET_KEY.
type EnvCloudflare struct {
	retrieved bool
}

func (m *EnvCloudflare) retrieve() (credentials.Value, error) {
	m.retrieved = false

	id := os.Getenv("CF_ACCESS_KEY")
	secret := os.Getenv("CF_SECRET_KEY")

	signerType := credentials.SignatureV4
	if id == "" || secret == "" {
		signerType = credentials.SignatureAnonymous
	}

	m.retrieved = true

	return credentials.Value{
		AccessKeyID:     id,
		SecretAccessKey: secret,
		SignerType:      signerType,
	}, nil
}

func (m *EnvCloudflare) Retrieve() (credentials.Value, error) {
	return m.retrieve()
}

func (m *EnvCloudflare) RetrieveWithCredContext(_ *credentials.CredContext) (credentials.Value, error) {
	return m.retrieve()
}

func (m *EnvCloudflare) IsExpired() bool {
	return !m.retrieved
}
