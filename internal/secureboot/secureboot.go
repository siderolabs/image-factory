// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package secureboot implements handling SecureBoot options.
package secureboot

import (
	"context"
	"encoding/pem"
	"fmt"
	"sync"

	"github.com/siderolabs/talos/pkg/imager/profile"
)

// Service handles SecureBoot configuration.
type Service struct { //nolint:govet
	in *profile.SecureBootAssets

	signingCertPEM []byte
	signingCertMu  sync.Mutex
}

// Options configures SecureBoot.
type Options struct { //nolint:govet
	// Enable SecureBoot asset generation.
	Enabled bool

	// File-based approach.
	SigningKeyPath, SigningCertPath string
	PCRKeyPath                      string

	// Azure Key Vault approach.
	AzureKeyVaultURL     string
	AzureCertificateName string
	AzureKeyName         string

	// AWS KMS approach.
	//
	// AWS KMS Key ID, ACM certificate ARN, and region.
	// Support local cert file for legacy use cases.
	AwsKMSKeyID    string
	AwsKMSPCRKeyID string
	AwsCertPath    string
	AwsCertARN     string
	AwsRegion      string
}

// ErrDisabled is returned when SecureBoot is disabled.
var ErrDisabled = fmt.Errorf("secure boot is disabled")

// NewService initializes SecureBoot from configuration.
func NewService(opts Options) (*Service, error) {
	if !opts.Enabled {
		return &Service{}, nil
	}

	switch {
	case opts.SigningKeyPath != "" && opts.SigningCertPath != "" && opts.PCRKeyPath != "":
		return &Service{
			in: &profile.SecureBootAssets{
				SecureBootSigner: profile.SigningKeyAndCertificate{
					KeyPath:  opts.SigningKeyPath,
					CertPath: opts.SigningCertPath,
				},
				PCRSigner: profile.SigningKey{
					KeyPath: opts.PCRKeyPath,
				},
			},
		}, nil
	case opts.AzureKeyVaultURL != "" && opts.AzureCertificateName != "" && opts.AzureKeyName != "":
		return &Service{
			in: &profile.SecureBootAssets{
				SecureBootSigner: profile.SigningKeyAndCertificate{
					AzureVaultURL:      opts.AzureKeyVaultURL,
					AzureCertificateID: opts.AzureCertificateName,
				},
				PCRSigner: profile.SigningKey{
					AzureVaultURL: opts.AzureKeyVaultURL,
					AzureKeyID:    opts.AzureKeyName,
				},
			},
		}, nil
	case opts.AwsKMSKeyID != "" && opts.AwsKMSPCRKeyID != "" && opts.AwsCertPath != "" && opts.AwsRegion != "":
		return &Service{
			in: &profile.SecureBootAssets{
				SecureBootSigner: profile.SigningKeyAndCertificate{
					AwsRegion:   opts.AwsRegion,
					AwsKMSKeyID: opts.AwsKMSKeyID,
					AwsCertPath: opts.AwsCertPath,
				},
				PCRSigner: profile.SigningKey{
					AwsRegion:   opts.AwsRegion,
					AwsKMSKeyID: opts.AwsKMSPCRKeyID,
				},
			},
		}, nil
	case opts.AwsKMSKeyID != "" && opts.AwsKMSPCRKeyID != "" && opts.AwsCertARN != "" && opts.AwsRegion != "":
		return &Service{
			in: &profile.SecureBootAssets{
				SecureBootSigner: profile.SigningKeyAndCertificate{
					AwsRegion:   opts.AwsRegion,
					AwsKMSKeyID: opts.AwsKMSKeyID,
					AwsCertARN:  opts.AwsCertARN,
				},
				PCRSigner: profile.SigningKey{
					AwsRegion:   opts.AwsRegion,
					AwsKMSKeyID: opts.AwsKMSPCRKeyID,
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("invalid SecureBoot configuration: %#+v", opts)
	}
}

// GetSecureBootAssets returns SecureBoot assets for the imager profile.
func (s *Service) GetSecureBootAssets() (*profile.SecureBootAssets, error) {
	if s.in == nil {
		// disabled
		return nil, ErrDisabled
	}

	return s.in, nil
}

// GetSecureBootSigningCert returns SecureBoot signing key PEM-encoded.
//
// Only a successful result is cached, so a failure is retried on the next call.
func (s *Service) GetSecureBootSigningCert() ([]byte, error) {
	if s.in == nil {
		return nil, ErrDisabled
	}

	s.signingCertMu.Lock()
	defer s.signingCertMu.Unlock()

	if s.signingCertPEM != nil {
		return s.signingCertPEM, nil
	}

	signer, err := s.in.SecureBootSigner.GetSigner(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get SecureBoot signing key: %w", err)
	}

	s.signingCertPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: signer.Certificate().Raw,
	})

	return s.signingCertPEM, nil
}
