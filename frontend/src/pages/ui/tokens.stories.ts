// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { apiError, apiPath, apiPending } from '@mocks/msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'
import { expect, waitFor } from 'storybook/test'

import type { Schemas } from '@/api/client'

import Tokens from './tokens.vue'

const meta: Meta<typeof Tokens> = {
  component: Tokens,
  parameters: {
    layout: 'fullscreen',
    route: '/ui/tokens',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const Empty: Story = {
  beforeEach({ msw }) {
    msw.use(
      http.get(apiPath('/tokens'), () =>
        HttpResponse.json<Schemas['APITokenList']>({ tokens: [] }),
      ),
    )
  },
}

/** A created token is shown once, with the patch that lets Talos pull images with it. */
export const Created: Story = {
  play: async ({ canvas, userEvent }) => {
    const create = canvas.getByRole('button', { name: 'Create Token' })

    // The button waits for the actors that tokens can be issued to.
    await waitFor(() => expect(create).toBeEnabled())
    await userEvent.click(create)
    await userEvent.type(await canvas.findByLabelText('Name'), 'worker-nodes{Enter}')
  },
}

export const ConfirmingRevoke: Story = {
  play: async ({ canvas, userEvent }) => {
    const [revoke] = await canvas.findAllByRole('button', { name: 'Revoke' })

    await userEvent.click(revoke)
  },
}

export const Loading: Story = {
  beforeEach({ msw }) {
    msw.use(apiPending('get', '/tokens'))
  },
}

export const Error: Story = {
  beforeEach({ msw }) {
    msw.use(apiError('get', '/tokens', 403, 'forbidden'))
  },
}
