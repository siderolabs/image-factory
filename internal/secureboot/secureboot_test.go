// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secureboot_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/secureboot"
)

func TestGetSecureBootSigningCertRetriesAfterFailure(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	keyPath := filepath.Join(directory, "key.pem")
	certPath := filepath.Join(directory, "cert.pem")

	service, err := secureboot.NewService(secureboot.Options{Enabled: true, SigningKeyPath: keyPath, SigningCertPath: certPath, PCRKeyPath: keyPath})
	require.NoError(t, err)

	// the key and the certificate are not there yet
	_, err = service.GetSecureBootSigningCert()
	require.Error(t, err)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotAfter: time.Now().Add(time.Hour)}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600))
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0o600))

	certificate, err := service.GetSecureBootSigningCert()
	require.NoError(t, err)
	require.Equal(t, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), certificate)
}

func TestGetSecureBootSigningCertDisabled(t *testing.T) {
	t.Parallel()

	service, err := secureboot.NewService(secureboot.Options{})
	require.NoError(t, err)

	_, err = service.GetSecureBootSigningCert()
	require.ErrorIs(t, err, secureboot.ErrDisabled)
}
