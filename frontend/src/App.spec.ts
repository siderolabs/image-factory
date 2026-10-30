// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { worker } from '@mocks/msw/worker'
import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { http } from 'msw'
import { beforeEach, describe, expect, test } from 'vitest'
import { render } from 'vitest-browser-vue'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'
import { routes } from 'vue-router/auto-routes'

import App from '@/App.vue'
import { i18n, initLanguage } from '@/i18n'
import { draft } from '@/wizard/draft'

/** Records the API requests that the wizard makes, leaving the mocked API to answer them. */
function recordRequests() {
  const requests: { method: string; url: URL; body: string }[] = []

  worker.use(
    http.all('*', async ({ request }) => {
      requests.push({
        method: request.method,
        url: new URL(request.url),
        body: await request.clone().text(),
      })
    }),
  )

  return requests
}

async function renderApp(
  path: string,
): Promise<{ router: Router; screen: Awaited<ReturnType<typeof render>> }> {
  const router = createRouter({ history: createMemoryHistory(), routes })

  await router.push(path)
  await initLanguage()

  const screen = await render(App, {
    global: { plugins: [router, i18n, [VueQueryPlugin, { queryClient: new QueryClient() }]] },
  })

  return { router, screen }
}

describe('wizard', () => {
  // The draft lives for the tab's session, which all the tests share.
  beforeEach(() => (draft.value = {}))

  test('builds a bare-metal schematic and links its downloads', async () => {
    const requests = recordRequests()
    const { router, screen } = await renderApp('/')
    const next = () => screen.getByRole('button', { name: 'Next' }).click()

    await expect.element(screen.getByRole('heading', { name: 'Hardware Type' })).toBeVisible()
    await next()

    // The latest stable version is picked, not the pre-release.
    await expect
      .element(screen.getByRole('combobox', { name: 'Choose Talos Linux Version' }))
      .toHaveValue('1.13.4')
    await next()

    await expect
      .element(screen.getByRole('heading', { name: 'Machine Architecture' }))
      .toBeVisible()
    await expect.element(screen.getByRole('checkbox', { name: 'SecureBoot' })).toBeInTheDocument()
    await next()

    await expect.element(screen.getByText('siderolabs/gvisor (20250505.0)')).toBeVisible()
    await expect.element(screen.getByText(/metal-agent/)).not.toBeInTheDocument()
    await screen.getByText('siderolabs/gvisor (20250505.0)').click()
    await next()

    await expect.element(screen.getByRole('heading', { name: 'Customization' })).toBeVisible()
    await screen.getByLabelText('Extra kernel command line arguments:').fill('console=ttyS0')
    await next()

    await expect.element(screen.getByRole('heading', { name: 'Schematic Ready' })).toBeVisible()

    expect(router.currentRoute.value.query).toEqual({
      target: 'metal',
      version: '1.13.4',
      arch: 'amd64',
      platform: 'metal',
      'schematic-id': expect.stringMatching(/^[0-9a-f]{64}$/),
    })

    const id = router.currentRoute.value.query['schematic-id']

    await expect
      .element(
        screen.getByText(`${location.origin}/image/${id}/v1.13.4/metal-amd64.iso`, { exact: true }),
      )
      .toBeVisible()
    await expect
      .element(screen.getByText(`${location.host}/metal-installer/${id}:v1.13.4`).first())
      .toBeVisible()

    const created = requests.find((r) => r.method === 'POST')

    expect(JSON.parse(created!.body)).toEqual({
      customization: {
        extraKernelArgs: ['console=ttyS0'],
        systemExtensions: { officialExtensions: ['siderolabs/gvisor'] },
      },
    })

    const assetsRequest = requests.find((r) => r.url.pathname.endsWith('/assets'))

    expect(Object.fromEntries(assetsRequest!.url.searchParams)).toEqual({
      target: 'metal',
      arch: 'amd64',
      secureboot: 'false',
    })
  })

  test('goes back to the previous choices', async () => {
    const { router, screen } = await renderApp('/?target=metal&version=1.13.4&platform=metal')

    await expect
      .element(screen.getByRole('heading', { name: 'Machine Architecture' }))
      .toBeVisible()
    await screen.getByRole('button', { name: 'Back' }).click()

    await expect
      .element(screen.getByRole('combobox', { name: 'Choose Talos Linux Version' }))
      .toHaveValue('1.13.4')
    expect(router.currentRoute.value.query).toEqual({ target: 'metal', platform: 'metal' })
  })
})
