// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package metadata provides meta information about supported Talos platforms, boards, etc.
package metadata

import (
	"github.com/blang/semver/v4"

	"github.com/skyssolutions/siderolabs-image-factory/internal/artifacts"
)

// Arch represents an architecture supported by Talos.
type Arch = artifacts.Arch

// Platform represents a platform supported by Talos.
type Platform struct {
	Name string

	Label       string
	Description string

	MinVersion          semver.Version
	Architectures       []Arch
	Documentation       string
	DiskImageSuffix     string
	BootMethods         []string
	SecureBootSupported bool
}

// NotOnlyDiskImage is true if the platform supports boot methods other than disk-image.
func (p Platform) NotOnlyDiskImage() bool {
	if len(p.BootMethods) == 1 && p.BootMethods[0] == "disk-image" {
		return false
	}

	return true
}

// Platforms returns a list of supported platforms.
func Platforms() []Platform {
	return []Platform{
		// Tier 1
		{
			Name: "aws",

			Label:       "Amazon Web Services (AWS)",
			Description: "Runs on AWS VMs booted from an AMI",

			Architectures:   []Arch{artifacts.ArchAmd64, artifacts.ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/aws/",
			DiskImageSuffix: "raw.xz",
			BootMethods: []string{
				"disk-image",
			},
		},
		{
			Name: "gcp",

			Label:       "Google Cloud (GCP)",
			Description: "Runs on Google Cloud VMs booted from a disk image",

			Architectures:   []Arch{artifacts.ArchAmd64, artifacts.ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/gcp/",
			DiskImageSuffix: "raw.tar.gz",
			BootMethods: []string{
				"disk-image",
			},
		},
		{
			Name: "equinixMetal",

			Label:       "Equinix Metal",
			Description: "Runs on Equinix Metal bare-metal servers",

			Architectures: []Arch{artifacts.ArchAmd64, artifacts.ArchArm64},
			Documentation: "/talos-guides/install/bare-metal-platforms/equinix-metal/",
			BootMethods: []string{
				"pxe",
			},
		},
		// metal platform is not listed here, as it is handled separately.
		// Tier 2
		{
			Name: "azure",

			Label:       "Microsoft Azure",
			Description: "Runs on Microsoft Azure Linux Virtual Machines",

			Architectures:   []Arch{artifacts.ArchAmd64, artifacts.ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/azure/",
			DiskImageSuffix: "vhd.xz",
			BootMethods: []string{
				"disk-image",
			},
		},
		{
			Name: "digital-ocean",

			Label:       "Digital Ocean",
			Description: "Runs on Digital Ocean droplets",

			Architectures:   []Arch{artifacts.ArchAmd64},
			Documentation:   "/talos-guides/install/cloud-platforms/digitalocean/",
			DiskImageSuffix: "raw.gz",
			BootMethods: []string{
				"disk-image",
			},
		},
		{
			Name: "openstack",

			Label:       "OpenStack",
			Description: "Runs on OpenStack virtual machines",

			Architectures:   []Arch{artifacts.ArchAmd64, artifacts.ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/openstack/",
			DiskImageSuffix: "raw.xz",
			BootMethods: []string{
				"disk-image",
				"iso",
				"pxe",
			},
		},
		{
			Name: "vmware",

			Label:       "VMWare",
			Description: "Runs on VMWare ESXi virtual machines",

			Architectures:   []Arch{artifacts.ArchAmd64},
			Documentation:   "/talos-guides/install/virtualized-platforms/vmware/",
			DiskImageSuffix: "ova",
			BootMethods: []string{
				"disk-image",
				"iso",
			},
		},
		// Tier 3
		{
			Name: "akamai",

			Label:       "Akamai",
			Description: "Runs on Akamai Cloud (Linode) virtual machines",

			Architectures:   []Arch{artifacts.ArchAmd64},
			MinVersion:      semver.MustParse("1.7.0"),
			Documentation:   "/talos-guides/install/cloud-platforms/akamai/",
			DiskImageSuffix: "raw.gz",
			BootMethods: []string{
				"disk-image",
			},
		},
		// Exoscale: no documentation on Talos side, skipping.
		{
			Name: "cloudstack",

			Label:       "Apache CloudStack",
			Description: "Runs on Apache CloudStack virtual machines",

			Architectures:   []Arch{artifacts.ArchAmd64, artifacts.ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/cloudstack/",
			DiskImageSuffix: "raw.gz",
			BootMethods: []string{
				"disk-image",
			},
			MinVersion: semver.MustParse("1.8.0-alpha.2"),
		},
		{
			Name: "hcloud",

			Label:       "Hetzner",
			Description: "Runs on Hetzner virtual machines",

			Architectures:   []Arch{artifacts.ArchAmd64},
			Documentation:   "/talos-guides/install/cloud-platforms/hetzner/",
			DiskImageSuffix: "raw.xz",
			BootMethods: []string{
				"disk-image",
			},
		},
		{
			Name: "nocloud",

			Label:       "Nocloud",
			Description: "Runs on various hypervisors supporting 'nocloud' metadata (Proxmox, Oxide Computer, etc.)",

			Architectures:   []Arch{artifacts.ArchAmd64, artifacts.ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/nocloud/",
			DiskImageSuffix: "raw.xz",
			BootMethods: []string{
				"disk-image",
				"iso",
				"pxe",
			},
			SecureBootSupported: true,
		},
		// OpenNebula: no documentation on Talos side, skipping.
		{
			Name: "oracle",

			Label:       "Oracle Cloud",
			Description: "Runs on Oracle Cloud virtual machines",

			Architectures:   []Arch{artifacts.ArchAmd64, artifacts.ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/oracle/",
			DiskImageSuffix: "raw.xz",
			BootMethods: []string{
				"disk-image",
			},
		},
		// Scaleway: no documentation on Talos side, skipping.
		{
			Name: "upcloud",

			Label:       "UpCloud",
			Description: "Runs on UpCloud virtual machines",

			Architectures:   []Arch{artifacts.ArchAmd64},
			Documentation:   "/talos-guides/install/cloud-platforms/ucloud/",
			DiskImageSuffix: "raw.xz",
			BootMethods: []string{
				"disk-image",
			},
		},
		{
			Name: "vultr",

			Label:       "Vultr",
			Description: "Runs on Vultr Cloud Compute virtual machines",

			Architectures: []Arch{artifacts.ArchAmd64},
			Documentation: "/talos-guides/install/cloud-platforms/vultr/",
			BootMethods: []string{
				"iso",
				"pxe",
			},
		},
	}
}
