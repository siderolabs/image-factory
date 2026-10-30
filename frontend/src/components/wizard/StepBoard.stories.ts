// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { apiError, apiPending } from '@mocks/msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import { draft } from '@/wizard/draft'

import StepBoard from './StepBoard.vue'

const meta: Meta<typeof StepBoard> = {
  component: StepBoard,
  args: {
    version: '1.13.4',
    onBack: fn(),
    onNext: fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

/** Starts from the Raspberry Pi Series. */
export const Default: Story = {}

/** Coming back to the step starts from the board picked before. */
export const ChosenBefore: Story = {
  beforeEach() {
    draft.value.board = 'rpi_5'
  },
}

export const Loading: Story = {
  beforeEach({ msw }) {
    msw.use(apiPending('get', '/api/version/{version}/boards'))
  },
}

export const Error: Story = {
  beforeEach({ msw }) {
    msw.use(apiError('get', '/api/version/{version}/boards', 400, 'invalid Talos version'))
  },
}
