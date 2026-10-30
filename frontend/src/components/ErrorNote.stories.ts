// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import type { Meta, StoryObj } from '@storybook/vue3-vite'

import ErrorNote from './ErrorNote.vue'

const meta: Meta<typeof ErrorNote> = {
  component: ErrorNote,
  args: {
    message: 'HTTP 502',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const ServerMessage: Story = {
  args: {
    message: 'error getting extensions: failed to pull ghcr.io/siderolabs/extensions:v1.13.4',
  },
}
