// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { apiError, apiPending } from '@mocks/msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import { draft } from '@/wizard/draft'

import StepExtensions from './StepExtensions.vue'

const meta: Meta<typeof StepExtensions> = {
  component: StepExtensions,
  args: {
    version: '1.13.4',
    onBack: fn(),
    onNext: fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

/** Coming back to the step starts from the extensions picked before. */
export const ChosenBefore: Story = {
  beforeEach() {
    draft.value.extensions = ['siderolabs/gvisor', 'siderolabs/iscsi-tools']
  },
}

/** The search keeps the extensions picked in view, whether they match or not. */
export const Searching: Story = {
  beforeEach() {
    draft.value.extensions = ['siderolabs/gvisor']
  },
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(await canvas.findByRole('searchbox'), 'ucode')
  },
}

export const Loading: Story = {
  beforeEach({ msw }) {
    msw.use(apiPending('get', '/version/{version}/extensions/official'))
  },
}

export const Error: Story = {
  beforeEach({ msw }) {
    msw.use(
      apiError(
        'get',
        '/version/{version}/extensions/official',
        500,
        'error getting extensions: failed to pull ghcr.io/siderolabs/extensions:v1.13.4',
      ),
    )
  },
}
