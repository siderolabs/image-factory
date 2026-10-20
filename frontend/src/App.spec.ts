// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { describe, expect, test } from 'vitest'
import { render } from 'vitest-browser-vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { routes } from 'vue-router/auto-routes'

import App from '@/App.vue'

describe('App', () => {
  test('renders the wizard page at the root', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes })

    await router.push('/')

    const screen = await render(App, { global: { plugins: [router] } })

    await expect.element(screen.getByRole('heading', { name: 'Image Factory' })).toBeVisible()
  })
})
