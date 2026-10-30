// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { vanillaSchematic } from '@mocks/fixtures'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import App from './App.vue'

const signedIn = { authenticated: true, logoutEnabled: true, tokensEnabled: true }

/** The whole app, on the mocked API. */
const meta: Meta<typeof App> = {
  component: App,
  parameters: {
    layout: 'fullscreen',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    config: { version: 'v1.0.0' },
  },
}

export const SchematicLink: Story = {
  parameters: {
    route: `/?target=metal&version=1.13.4&arch=amd64&platform=metal&schematic-id=${vanillaSchematic.id}`,
  },
}

export const Enterprise: Story = {
  parameters: {
    config: { enterprise: true, version: 'v1.0.0', ...signedIn },
    route: SchematicLink.parameters?.route,
  },
}

export const EnterpriseSignedOut: Story = {
  parameters: {
    config: { enterprise: true, version: 'v1.0.0', logoutEnabled: true },
  },
}

export const Tokens: Story = {
  parameters: {
    config: { enterprise: true, version: 'v1.0.0', ...signedIn },
    route: '/ui/tokens',
  },
}

export const SignInError: Story = {
  parameters: {
    config: { enterprise: true, version: 'v1.0.0', logoutEnabled: true },
    route: '/sign-in-error?reason=cookie',
  },
}
