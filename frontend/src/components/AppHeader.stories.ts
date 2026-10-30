// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import type { Meta, StoryObj } from '@storybook/vue3-vite'

import AppHeader from './AppHeader.vue'

const signedIn = { authenticated: true, logoutEnabled: true, tokensEnabled: true }

const meta: Meta<typeof AppHeader> = {
  component: AppHeader,
  parameters: {
    layout: 'fullscreen',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const Enterprise: Story = {
  parameters: {
    config: { enterprise: true },
  },
}

export const SignedIn: Story = {
  parameters: {
    config: { enterprise: true, ...signedIn },
  },
}

export const OnTokensPage: Story = {
  parameters: {
    config: { enterprise: true, ...signedIn },
    route: '/ui/tokens',
  },
}

/** Signed in with a browser login, on a factory that doesn't issue tokens. */
export const WithoutTokens: Story = {
  parameters: {
    config: { enterprise: true, ...signedIn, tokensEnabled: false },
  },
}
