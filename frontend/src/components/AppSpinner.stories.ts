// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import type { Meta, StoryObj } from '@storybook/vue3-vite'

import AppSpinner from './AppSpinner.vue'

const meta: Meta<typeof AppSpinner> = {
  component: AppSpinner,
  parameters: {
    layout: 'centered',
  },
  // The spinner takes its size from the class it's given, as the wizard steps do.
  render: () => ({
    components: { AppSpinner },
    template: '<AppSpinner class="size-8" />',
  }),
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
