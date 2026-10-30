// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { faker } from '@faker-js/faker'
import { addDays } from 'date-fns'

import type { Schemas } from '@/api/client'
import { config } from '@/config'
import type { Arch, Target } from '@/wizard/params'
import type { ScanReport } from '@/wizard/scan'

export const versions: Schemas['VersionList'] = [
  'v1.11.5',
  'v1.12.0',
  'v1.12.6',
  'v1.13.0',
  'v1.13.4',
  'v1.14.0-alpha.0',
]

export const features: Schemas['VersionFeatures'] = [
  'SecureBoot',
  'BootloaderSelection',
  'EmbeddedConfig',
  'TalosctlDownload',
  'LocalTestCluster',
  'SBOM',
  'VEX',
  'ProductionGuide',
  'TroubleshootingGuide',
]

const metal: Schemas['Platform'] = {
  name: 'metal',
  label: 'Bare Metal',
  description: 'Runs on bare-metal servers',
  documentation: '/platform-specific-installations/bare-metal-platforms/bootloader',
  architectures: ['amd64', 'arm64'],
  bootMethods: ['iso', 'disk-image', 'pxe'],
  secureBootSupported: true,
  notOnlyDiskImage: true,
}

export const platforms: Schemas['Platform'][] = [
  {
    name: 'aws',
    label: 'Amazon Web Services (AWS)',
    description: 'Runs on AWS VMs booted from an AMI',
    documentation: '/platform-specific-installations/cloud-platforms/aws',
    architectures: ['amd64', 'arm64'],
    bootMethods: ['disk-image'],
    secureBootSupported: false,
    notOnlyDiskImage: false,
  },
  {
    name: 'gcp',
    label: 'Google Cloud (GCP)',
    description: 'Runs on Google Cloud VMs booted from a disk image',
    documentation: '/platform-specific-installations/cloud-platforms/gcp',
    architectures: ['amd64', 'arm64'],
    bootMethods: ['disk-image'],
    secureBootSupported: false,
    notOnlyDiskImage: false,
  },
  {
    name: 'azure',
    label: 'Microsoft Azure',
    description: 'Runs on Microsoft Azure Linux Virtual Machines',
    documentation: '/platform-specific-installations/cloud-platforms/azure',
    architectures: ['amd64', 'arm64'],
    bootMethods: ['disk-image'],
    secureBootSupported: false,
    notOnlyDiskImage: false,
  },
  {
    name: 'digital-ocean',
    label: 'Digital Ocean',
    description: 'Runs on Digital Ocean droplets',
    documentation: '/platform-specific-installations/cloud-platforms/digitalocean',
    architectures: ['amd64'],
    bootMethods: ['disk-image'],
    secureBootSupported: false,
    notOnlyDiskImage: false,
  },
  {
    name: 'nocloud',
    label: 'Nocloud',
    description:
      "Runs on various hypervisors supporting 'nocloud' metadata (Proxmox, Oxide Computer, etc.)",
    documentation: '/platform-specific-installations/cloud-platforms/nocloud',
    architectures: ['amd64', 'arm64'],
    bootMethods: ['disk-image', 'iso', 'pxe'],
    secureBootSupported: true,
    notOnlyDiskImage: true,
  },
  {
    name: 'vmware',
    label: 'VMWare',
    description: 'Runs on VMWare ESXi virtual machines',
    documentation: '/platform-specific-installations/virtualized-platforms/vmware',
    architectures: ['amd64', 'arm64'],
    bootMethods: ['disk-image', 'iso'],
    secureBootSupported: false,
    notOnlyDiskImage: true,
  },
]

export const boards: Schemas['Board'][] = [
  {
    name: 'rpi_5',
    label: 'Raspberry Pi 5',
    documentation: '',
    overlay: { name: 'rpi_5', image: 'siderolabs/sbc-raspberrypi' },
  },
  {
    name: 'rpi_generic',
    label: 'Raspberry Pi Series',
    documentation: '/platform-specific-installations/single-board-computers/rpi_generic',
    overlay: { name: 'rpi_generic', image: 'siderolabs/sbc-raspberrypi' },
  },
  {
    name: 'bananapi_m64',
    label: 'Banana Pi M64',
    documentation: '/platform-specific-installations/single-board-computers/bananapi_m64',
    overlay: { name: 'bananapi_m64', image: 'siderolabs/sbc-allwinner' },
  },
  {
    name: 'nanopi_r4s',
    label: 'Friendlyelec Nano PI R4S',
    documentation: '/platform-specific-installations/single-board-computers/nanopi_r4s',
    overlay: { name: 'nanopi-r4s', image: 'siderolabs/sbc-rockchip' },
  },
]

/** The official extensions, including the internal metal-agent that the UI hides. */
export const extensions: Schemas['Extension'][] = [
  {
    name: 'siderolabs/amd-ucode',
    ref: 'ghcr.io/siderolabs/amd-ucode:20250509',
    digest: 'sha256:237bd8cbb3e8bdc4e34e61de2c820e71ea466ae6c106167fb324ced0caf57ffb',
    author: 'Sidero Labs',
    description: 'This system extension provides AMD microcode binaries.',
  },
  {
    name: 'siderolabs/gvisor',
    ref: 'ghcr.io/siderolabs/gvisor:20250505.0',
    digest: 'sha256:3e9d0ddac5ac9701e0dc38a9d09ffc17a5cbbe93e859905fdbaa5659cbf5f7f5',
    author: 'Sidero Labs',
    description: 'This system extension provides gVisor using containerd runtime handler.',
  },
  {
    name: 'siderolabs/intel-ucode',
    ref: 'ghcr.io/siderolabs/intel-ucode:20250512',
    digest: 'sha256:086a842885f85a26bdc525e80ec5b237cef265d729c44b48154ba2878036a05b',
    author: 'Sidero Labs',
    description: 'This system extension provides Intel microcode binaries.',
  },
  {
    name: 'siderolabs/iscsi-tools',
    ref: 'ghcr.io/siderolabs/iscsi-tools:v0.2.0',
    digest: 'sha256:78d3592681eff0a5eddb4b3a2a391a3ed62a88c12056953c322982aa4b792596',
    author: 'Sidero Labs',
    description: 'This system extension provides iscsi-tools.',
  },
  {
    name: 'siderolabs/metal-agent',
    ref: 'ghcr.io/siderolabs/metal-agent:v0.1.3',
    digest: 'sha256:923d48122b454873542bb80f561df21aa50fa68201a16b4ddb492d01c434bc15',
    author: 'Sidero Labs',
    description: 'This system extension provides the Omni metal agent.',
  },
  {
    name: 'siderolabs/qemu-guest-agent',
    ref: 'ghcr.io/siderolabs/qemu-guest-agent:10.0.0',
    digest: 'sha256:35359b60bec5d87abc3247df200a5921d0291db65280776931bcf663bf63458f',
    author: 'Markus Reiter',
    description: 'This system extension provides the QEMU Guest Agent service.',
  },
  {
    name: 'siderolabs/util-linux-tools',
    ref: 'ghcr.io/siderolabs/util-linux-tools:2.41.0',
    digest: 'sha256:d98d3de232a24b5e8465052c15ab165f810a903c39c302c329ff216e4b1c1015',
    author: 'Sidero Labs',
    description: 'This system extension provides util-linux tools.',
  },
]

/** The schematic without customization, which the factory always knows. */
export const vanillaSchematic: Schemas['SchematicCreated'] = {
  id: '376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba',
  schematic: 'customization: {}\n',
}

export const tokenActors: Schemas['APITokenActor'][] = [
  { id: 'talos', scopes: ['image:read'], issuable_scopes: [] },
  { id: 'onprem', scopes: ['source:pull'], issuable_scopes: [] },
  {
    id: 'automation',
    scopes: ['image:read', 'report:read', 'schematic:create', 'schematic:read', 'token:issue'],
    issuable_scopes: [
      'image:read',
      'report:read',
      'schematic:create',
      'schematic:read',
      'token:issue',
    ],
  },
  {
    id: 'operator',
    scopes: ['image:read', 'report:read', 'schematic:create', 'schematic:read', 'source:pull'],
    issuable_scopes: [],
  },
  {
    id: 'admin',
    scopes: [
      'image:read',
      'report:read',
      'schematic:create',
      'schematic:read',
      'source:pull',
      'token:issue',
      'token:read',
      'token:revoke',
    ],
    issuable_scopes: [
      'image:read',
      'report:read',
      'schematic:create',
      'schematic:read',
      'source:pull',
      'token:issue',
      'token:read',
      'token:revoke',
    ],
  },
]

/** A stored API token, issued to a random actor. */
export function fakeToken(): Schemas['APIToken'] {
  const actor = faker.helpers.arrayElement(tokenActors)
  const created = faker.date.recent({ days: 60 })

  return {
    id: faker.string.ulid({ refDate: created }),
    name: `${faker.hacker.adjective()}-${faker.hacker.noun()}`.replaceAll(' ', '-'),
    created_at: created.toISOString(),
    expires_at: addDays(created, 90).toISOString(),
    scopes: actor.scopes,
    issuable_scopes: actor.issuable_scopes.length ? actor.issuable_scopes : undefined,
  }
}

/** A grype report with a match of each severity, as the vulnerability scan of an image. */
export const scanReport: ScanReport = {
  matches: [
    {
      vulnerability: {
        id: 'CVE-2025-21756',
        severity: 'Critical',
        description: 'vsock: Keep the binding until socket destruction.',
        dataSource: 'https://nvd.nist.gov/vuln/detail/CVE-2025-21756',
        fix: { versions: ['6.12.16'] },
      },
      artifact: { name: 'linux', version: '6.12.13' },
    },
    {
      vulnerability: {
        id: 'GHSA-v778-237x-gjrc',
        severity: 'High',
        description: 'Misuse of ServerConfig.PublicKeyCallback may cause authorization bypass.',
        dataSource: 'https://github.com/advisories/GHSA-v778-237x-gjrc',
        fix: { versions: ['0.31.0'] },
      },
      relatedVulnerabilities: [
        {
          id: 'CVE-2024-45337',
          dataSource: 'https://nvd.nist.gov/vuln/detail/CVE-2024-45337',
        },
      ],
      artifact: { name: 'golang.org/x/crypto', version: 'v0.29.0' },
    },
    {
      vulnerability: {
        id: 'CVE-2025-22869',
        severity: 'Medium',
        description: 'SSH servers which implement file transfer protocols are vulnerable to DoS.',
        dataSource: 'https://nvd.nist.gov/vuln/detail/CVE-2025-22869',
      },
      artifact: { name: 'golang.org/x/crypto', version: 'v0.29.0' },
      matchDetails: [
        {
          matcher: 'go-module-matcher',
          found: {
            Statement: {
              status: 'not_affected',
              justification: 'vulnerable_code_not_in_execute_path',
              impact_statement: 'Talos does not run an SSH server.',
            },
          },
        },
      ],
    },
    {
      vulnerability: {
        id: 'CVE-2024-12797',
        severity: 'Low',
        description: 'RFC7250 handshakes with unauthenticated servers do not abort as expected.',
        dataSource: 'https://nvd.nist.gov/vuln/detail/CVE-2024-12797',
        fix: { versions: ['3.4.1'] },
      },
      artifact: { name: 'openssl', version: '3.4.0' },
    },
  ],
}

/** What the assets of a schematic are requested for, from the query of the request. */
export interface AssetsRequest {
  schematic: string
  version: string
  target: Target
  arch: Arch
  platform?: string
  board?: string
  secureBoot: boolean
}

/**
 * The downloads of a schematic, built the way the factory builds them. Like the factory, it adds checksums,
 * signatures, the SBOM, VEX and scan only on Enterprise, which stories turn on with the config parameter.
 */
export function schematicAssets(
  req: AssetsRequest,
  enterprise = config.enterprise,
): Schemas['SchematicAssets'] {
  const origin = location.origin
  const version = `v${req.version}`
  const imageBase = `${origin}/image/${req.schematic}/${version}`
  const secureBoot = req.secureBoot ? '-secureboot' : ''

  const image = (
    kind: Schemas['Asset']['kind'],
    path: string,
    format?: Schemas['Asset']['format'],
  ): Schemas['Asset'] => ({
    kind,
    format,
    secureBoot: req.secureBoot,
    url: `${imageBase}/${path}`,
    ...(enterprise && {
      checksums: { sha256: `${imageBase}/${path}.sha256`, sha512: `${imageBase}/${path}.sha512` },
      signature: `${imageBase}/${path}.sig`,
    }),
  })

  let bootAssets: Schemas['Asset'][]
  let extraAssets: Schemas['Asset'][] = []

  if (req.target === 'sbc') {
    bootAssets = [image('board-image', 'metal-arm64.raw.xz')]
  } else {
    const platform = req.target === 'cloud' ? platforms.find((p) => p.name === req.platform) : metal
    const name = platform?.name ?? req.platform
    const prefix = `${name}-${req.arch}${secureBoot}`

    bootAssets = (platform?.bootMethods ?? []).flatMap((method): Schemas['Asset'][] => {
      switch (method) {
        case 'disk-image':
          return req.target === 'metal' && !req.secureBoot
            ? [
                image('disk-image', `${prefix}.raw.zst`, 'raw'),
                image('disk-image', `${prefix}.qcow2`, 'qcow2'),
              ]
            : [image('disk-image', `${prefix}.raw.xz`)]
        case 'iso':
          return [image('iso', `${prefix}.iso`)]
        case 'pxe':
          return [
            {
              kind: 'pxe',
              secureBoot: req.secureBoot,
              url: `${origin}/pxe/${req.schematic}/${version}/${prefix}`,
            },
          ]
      }
    })

    if (req.target === 'metal') {
      extraAssets = req.secureBoot
        ? [image('uki', `${prefix}-uki.efi`)]
        : [
            image('kernel', `kernel-${req.arch}`),
            image('cmdline', `cmdline-metal-${req.arch}`),
            image('initramfs', `initramfs-${req.arch}.xz`),
            image('uki', `metal-${req.arch}-uki.efi`),
          ]
    }
  }

  const installerPlatform = req.target === 'cloud' ? req.platform : 'metal'
  const installer = `${installerPlatform}-installer${secureBoot}`

  return {
    bootAssets,
    extraAssets,
    talosctl: ['darwin-amd64', 'darwin-arm64', 'linux-amd64', 'linux-arm64', 'windows-amd64.exe']
      .map((tuple) => `talosctl-${tuple}`)
      .map((name) => ({ name, url: `${origin}/talosctl/${version}/${name}` })),
    installerImage: `${location.host}/${installer}/${req.schematic}:${version}`,
    ...(enterprise && {
      sbom: `${origin}/spdx/${req.schematic}/${version}/${req.arch}`,
      vex: `${origin}/vex/${version}/vex.json`,
      scan: `${origin}/scans/${req.schematic}/${version}/${req.arch}`,
    }),
  }
}
