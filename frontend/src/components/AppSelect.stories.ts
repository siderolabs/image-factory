// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { versions } from '@mocks/fixtures'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import AppSelect from './AppSelect.vue'

const meta: Meta<typeof AppSelect> = {
  component: AppSelect,
  args: {
    modelValue: 'v1.13.4',
    'onUpdate:modelValue': fn(),
  },
  render: (args) => ({
    components: { AppSelect },
    setup: () => ({ args, versions }),
    template: `
      <AppSelect v-bind="args">
        <option v-for="version in versions" :key="version" :value="version">{{ version }}</option>
      </AppSelect>
    `,
  }),
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const Dense: Story = {
  args: {
    dense: true,
  },
}
