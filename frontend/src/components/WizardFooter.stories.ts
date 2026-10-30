// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import WizardFooter from './WizardFooter.vue'

const meta: Meta<typeof WizardFooter> = {
  component: WizardFooter,
  args: {
    back: true,
    next: true,
    onBack: fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

/** The first step has nothing to go back to. */
export const NextOnly: Story = {
  args: {
    back: false,
  },
}

/** The final step has nothing to go on to. */
export const BackOnly: Story = {
  args: {
    next: false,
  },
}

export const Busy: Story = {
  args: {
    busy: true,
  },
}
