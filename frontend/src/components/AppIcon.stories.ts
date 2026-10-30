// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import type { Meta, StoryObj } from '@storybook/vue3-vite'

import AppIcon from './AppIcon.vue'

const names = [
  'arrow',
  'copy',
  'sun',
  'moon',
  'monitor',
  'check',
  'search',
  'external',
  'plus',
] as const

const meta: Meta<typeof AppIcon> = {
  component: AppIcon,
  args: {
    name: 'arrow',
  },
  argTypes: {
    name: { control: 'select', options: names },
  },
  parameters: {
    layout: 'centered',
  },
  // The icons take their size from the class they're given, and their colour from the text.
  render: (args) => ({
    components: { AppIcon },
    setup: () => ({ args }),
    template: '<AppIcon v-bind="args" class="size-6 text-content-emphasis" />',
  }),
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const All: Story = {
  render: () => ({
    components: { AppIcon },
    setup: () => ({ names }),
    template: `
      <ul class="grid grid-cols-3 gap-base text-content-emphasis">
        <li v-for="name in names" :key="name" class="flex flex-col items-center gap-tight type-meta">
          <AppIcon :name="name" class="size-6" />
          {{ name }}
        </li>
      </ul>
    `,
  }),
}
