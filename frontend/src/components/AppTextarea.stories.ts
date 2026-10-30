// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import AppTextarea from './AppTextarea.vue'

const meta: Meta<typeof AppTextarea> = {
  component: AppTextarea,
  args: {
    modelValue: '',
    'onUpdate:modelValue': fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const WithValue: Story = {
  args: {
    modelValue: `apiVersion: v1alpha1
kind: HostnameConfig
hostname: worker-1
`,
  },
}
