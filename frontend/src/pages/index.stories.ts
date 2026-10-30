// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import type { Meta, StoryObj } from '@storybook/vue3-vite'

import Index from './index.vue'

const meta: Meta<typeof Index> = {
  component: Index,
  parameters: {
    layout: 'fullscreen',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

/** With a browser login, the wizard waits for the user to sign in. */
export const SignedOut: Story = {
  parameters: {
    config: { logoutEnabled: true, authenticated: false },
  },
}
