// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { apiError, apiPending } from '@mocks/msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import { draft } from '@/wizard/draft'

import StepVersion from './StepVersion.vue'

const meta: Meta<typeof StepVersion> = {
  component: StepVersion,
  args: {
    target: 'metal',
    onBack: fn(),
    onNext: fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

/** Starts from the latest stable version. */
export const Default: Story = {}

/** Coming back to the step starts from the version picked before. */
export const ChosenBefore: Story = {
  beforeEach() {
    draft.value.version = '1.12.6'
  },
}

export const Loading: Story = {
  beforeEach({ msw }) {
    msw.use(apiPending('get', '/versions'))
  },
}

export const Error: Story = {
  beforeEach({ msw }) {
    msw.use(apiError('get', '/versions', 500, 'failed to list Talos versions'))
  },
}
