// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { type AssetsRequest, schematicAssets, vanillaSchematic } from '@mocks/fixtures'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { Schemas } from '@/api/client'

import AssetItem from './AssetItem.vue'

const version = '1.13.4'

/** The first asset of a kind, of the downloads the factory lists for a request. */
function asset(
  kind: Schemas['Asset']['kind'],
  request: Partial<AssetsRequest> = {},
  enterprise = false,
) {
  const { bootAssets, extraAssets } = schematicAssets(
    {
      schematic: vanillaSchematic.id,
      version,
      target: 'metal',
      arch: 'amd64',
      secureBoot: false,
      ...request,
    },
    enterprise,
  )

  return [...bootAssets, ...extraAssets].find((a) => a.kind === kind)!
}

const meta: Meta<typeof AssetItem> = {
  component: AssetItem,
  args: {
    asset: asset('iso'),
    target: 'metal',
    version,
  },
  // The assets are items of the download list on the final step.
  render: (args) => ({
    components: { AssetItem },
    setup: () => ({ args }),
    template: '<ul class="list-disc ps-base"><AssetItem v-bind="args" /></ul>',
  }),
}

export default meta
type Story = StoryObj<typeof meta>

export const ISO: Story = {}

export const SecureBootISO: Story = {
  args: {
    asset: asset('iso', { secureBoot: true }),
  },
}

export const DiskImage: Story = {
  args: {
    asset: asset('disk-image'),
  },
}

export const PXE: Story = {
  args: {
    asset: asset('pxe'),
  },
}

export const Kernel: Story = {
  args: {
    asset: asset('kernel'),
  },
}

export const CloudDiskImage: Story = {
  args: {
    asset: asset('disk-image', { target: 'cloud', platform: 'aws' }),
    target: 'cloud',
  },
}

export const BoardImage: Story = {
  args: {
    asset: asset('board-image', { target: 'sbc', board: 'rpi_generic', arch: 'arm64' }),
    target: 'sbc',
  },
}

/** Enterprise lists the checksums and signature of each asset. */
export const Enterprise: Story = {
  args: {
    asset: asset('iso', {}, true),
  },
  parameters: {
    config: { enterprise: true },
  },
}
