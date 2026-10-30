// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { apiError, apiPath, apiPending } from '@mocks/msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'

import type { Schemas } from '@/api/client'
import { draft } from '@/wizard/draft'

import StepCustomization from './StepCustomization.vue'

const meta: Meta<typeof StepCustomization> = {
  component: StepCustomization,
  args: {
    target: 'metal',
    version: '1.13.4',
    secureBoot: false,
    onBack: fn(),
    onNext: fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Metal: Story = {}

/** GRUB can't boot SecureBoot images, so it isn't offered. */
export const SecureBoot: Story = {
  args: {
    secureBoot: true,
  },
}

/** A single-board computer also takes the options of its board's overlay. */
export const SingleBoardComputer: Story = {
  args: {
    target: 'sbc',
    board: 'rpi_generic',
  },
}

export const InvalidOverlayOptions: Story = {
  args: {
    target: 'sbc',
    board: 'rpi_generic',
  },
  beforeEach() {
    draft.value.overlayOptions = '- configTxtAppend: dtoverlay=disable-bt'
  },
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole('button', { name: 'Next' }))
  },
}

/** A Talos version before embedded configuration and bootloader selection offers neither. */
export const OlderVersion: Story = {
  args: {
    version: '1.9.5',
  },
  beforeEach({ msw }) {
    msw.use(
      http.get(apiPath('/api/version/{version}/features'), () =>
        HttpResponse.json<Schemas['VersionFeatures']>(['SecureBoot', 'LocalTestCluster']),
      ),
    )
  },
}

/** Coming back to the step starts from the choices made before. */
export const ChosenBefore: Story = {
  beforeEach() {
    Object.assign(draft.value, {
      cmdline: 'console=ttyS0 net.ifnames=0',
      bootloader: 'grub',
      embeddedConfig: `apiVersion: v1alpha1
kind: HostnameConfig
hostname: worker-1
`,
    })
  },
}

export const Loading: Story = {
  beforeEach({ msw }) {
    msw.use(apiPending('get', '/api/version/{version}/features'))
  },
}

export const Error: Story = {
  beforeEach({ msw }) {
    msw.use(apiError('get', '/api/version/{version}/features', 400, 'invalid Talos version'))
  },
}
