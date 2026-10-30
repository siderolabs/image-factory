// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { features } from '@mocks/fixtures'
import { apiError, apiPath, apiPending } from '@mocks/msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'

import type { Schemas } from '@/api/client'

import StepArch from './StepArch.vue'

const meta: Meta<typeof StepArch> = {
  component: StepArch,
  args: {
    target: 'metal',
    version: '1.13.4',
    onBack: fn(),
    onNext: fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

/** Bare metal always supports SecureBoot. */
export const Metal: Story = {}

/** AWS doesn't support SecureBoot, so it isn't offered. */
export const Cloud: Story = {
  args: {
    target: 'cloud',
    platform: 'aws',
  },
}

export const CloudWithSecureBoot: Story = {
  args: {
    target: 'cloud',
    platform: 'nocloud',
  },
}

/** A Talos version without SecureBoot doesn't offer it, even on bare metal. */
export const VersionWithoutSecureBoot: Story = {
  beforeEach({ msw }) {
    msw.use(
      http.get(apiPath('/api/version/{version}/features'), () =>
        HttpResponse.json<Schemas['VersionFeatures']>(features.filter((f) => f !== 'SecureBoot')),
      ),
    )
  },
}

export const Loading: Story = {
  beforeEach({ msw }) {
    msw.use(apiPending('get', '/api/version/{version}/features'))
  },
}

export const Error: Story = {
  beforeEach({ msw }) {
    msw.use(apiError('get', '/api/version/{version}/features', 400, 'invalid Talos version'))
  },
}
