// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import RadioCard from './RadioCard.vue'

const meta: Meta<typeof RadioCard> = {
  component: RadioCard,
  args: {
    name: 'target',
    value: 'metal',
    label: 'Bare-metal Machine',
    modelValue: 'cloud',
    'onUpdate:modelValue': fn(),
  },
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const Checked: Story = {
  args: {
    modelValue: 'metal',
  },
}

export const Mono: Story = {
  args: {
    name: 'arch',
    value: 'arm64',
    label: 'arm64',
    mono: true,
  },
}
