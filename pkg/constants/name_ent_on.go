// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build enterprise

package constants

const (
	// TalosName is the name in the profile.
	TalosName = "Talos Enterprise"
	// ImageFactoryName is the name of the image factory.
	ImageFactoryName = "Image Factory Enterprise"
	// TalosPackageName is the name of the Talos package in SPDX documents.
	TalosPackageName = "talos-enterprise"
	// TalosPURL is the purl for Talos Enterprise.
	TalosPURL = "pkg:generic/" + TalosPackageName
)
