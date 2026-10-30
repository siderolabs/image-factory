// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import type { Meta, StoryObj } from '@storybook/vue3-vite'

import SignInError from './sign-in-error.vue'

/** The backend sends the browser here with the reason the sign-in failed. */
const meta: Meta<typeof SignInError> = {
  component: SignInError,
  parameters: {
    layout: 'fullscreen',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Unknown: Story = {
  parameters: {
    route: '/sign-in-error',
  },
}

export const Cookie: Story = {
  parameters: {
    route: '/sign-in-error?reason=cookie',
  },
}

export const Rejected: Story = {
  parameters: {
    route: '/sign-in-error?reason=rejected',
  },
}

export const Account: Story = {
  parameters: {
    route: '/sign-in-error?reason=account',
  },
}
