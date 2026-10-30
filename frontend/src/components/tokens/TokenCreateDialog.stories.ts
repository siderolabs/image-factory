// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { tokenActors } from '@mocks/fixtures'
import { apiError, apiPath, apiPending } from '@mocks/msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'

import TokenCreateDialog from './TokenCreateDialog.vue'

const meta: Meta<typeof TokenCreateDialog> = {
  component: TokenCreateDialog,
  args: {
    actors: tokenActors,
    onClose: fn(),
    onCreated: fn(),
  },
  parameters: {
    layout: 'fullscreen',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

/** The lifetime shows the date it works out to. */
export const Filled: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(canvas.getByLabelText('Name'), 'ci-pipeline')
    await userEvent.type(canvas.getByLabelText('Lifetime'), '720h')
    await userEvent.click(canvas.getByText('Automation (Omni / Terraform)'))
  },
}

export const Creating: Story = {
  beforeEach({ msw }) {
    msw.use(apiPending('post', '/tokens'))
  },
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(canvas.getByLabelText('Name'), 'ci-pipeline{Enter}')
  },
}

/** The identity has as many tokens as it may. */
export const LimitReached: Story = {
  beforeEach({ msw }) {
    msw.use(http.post(apiPath('/tokens'), () => new HttpResponse(null, { status: 409 })))
  },
  play: Creating.play,
}

export const Error: Story = {
  beforeEach({ msw }) {
    msw.use(apiError('post', '/tokens', 400, 'apitoken: requested TTL out of range'))
  },
  play: Creating.play,
}
