// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { vanillaSchematic } from '@mocks/fixtures'
import { apiError, apiPath, apiPending } from '@mocks/msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { ScanReport } from '@/wizard/scan'

import ScanSummary from './ScanSummary.vue'

const meta: Meta<typeof ScanSummary> = {
  component: ScanSummary,
  args: {
    scan: `${location.origin}/scans/${vanillaSchematic.id}/v1.13.4/amd64`,
  },
  parameters: {
    config: { enterprise: true },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const Vulnerabilities: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole('button', { name: /Critical/ }))
  },
}

export const NoVulnerabilities: Story = {
  beforeEach({ msw }) {
    msw.use(
      http.get(apiPath('/scans/{schematic}/{version}/{arch}/{report}'), () =>
        HttpResponse.json<ScanReport>({ matches: [] }),
      ),
    )
  },
}

/** Outside Enterprise, there's no scan, and the summary tells where to get one. */
export const NotEnterprise: Story = {
  args: {
    scan: undefined,
  },
  parameters: {
    config: { enterprise: false },
  },
}

export const Loading: Story = {
  beforeEach({ msw }) {
    msw.use(apiPending('get', '/scans/{schematic}/{version}/{arch}/{report}'))
  },
}

export const Error: Story = {
  beforeEach({ msw }) {
    msw.use(apiError('get', '/scans/{schematic}/{version}/{arch}/{report}', 404, 'not found'))
  },
}
