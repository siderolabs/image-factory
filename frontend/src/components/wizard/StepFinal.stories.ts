// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { vanillaSchematic } from '@mocks/fixtures'
import { apiError, apiPending } from '@mocks/msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import StepFinal from './StepFinal.vue'

const meta: Meta<typeof StepFinal> = {
  component: StepFinal,
  args: {
    schematicId: vanillaSchematic.id,
    target: 'metal',
    version: '1.13.4',
    arch: 'amd64',
    secureBoot: false,
    onBack: fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Metal: Story = {}

export const MetalSecureBoot: Story = {
  args: {
    secureBoot: true,
  },
}

export const Cloud: Story = {
  args: {
    target: 'cloud',
    platform: 'aws',
    arch: 'arm64',
  },
}

/** A cloud platform that also boots from an ISO or over PXE. */
export const CloudNotOnlyDiskImage: Story = {
  args: {
    target: 'cloud',
    platform: 'nocloud',
  },
}

export const SingleBoardComputer: Story = {
  args: {
    target: 'sbc',
    board: 'rpi_generic',
    arch: 'arm64',
  },
}

/** Enterprise adds checksums and signatures, the SBOM, VEX and the vulnerability scan. */
export const Enterprise: Story = {
  parameters: {
    config: { enterprise: true },
  },
}

export const Loading: Story = {
  beforeEach({ msw }) {
    msw.use(apiPending('get', '/api/schematics/{schematic}/{version}/assets'))
  },
}

export const Error: Story = {
  beforeEach({ msw }) {
    msw.use(
      apiError(
        'get',
        '/api/schematics/{schematic}/{version}/assets',
        400,
        'architecture "amd64" is not supported on single-board computers',
      ),
    )
  },
}
