// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import { draft } from '@/wizard/draft'

import StepTarget from './StepTarget.vue'

const meta: Meta<typeof StepTarget> = {
  component: StepTarget,
  args: {
    onNext: fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

/** Coming back to the step starts from the choice made before. */
export const ChosenBefore: Story = {
  beforeEach() {
    draft.value.target = 'sbc'
  },
}
