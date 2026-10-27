// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { render } from 'vitest-browser-vue'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'
import { routes } from 'vue-router/auto-routes'

import App from '@/App.vue'
import { i18n, initLanguage } from '@/i18n'
import { draft } from '@/wizard/draft'

const schematicYAML = `customization:
  systemExtensions:
    officialExtensions:
      - siderolabs/gvisor
`

const features = [
  'SecureBoot',
  'BootloaderSelection',
  'EmbeddedConfig',
  'LocalTestCluster',
  'ProductionGuide',
  'TroubleshootingGuide',
]

const assets = {
  bootAssets: [
    {
      kind: 'iso',
      secureBoot: false,
      url: 'https://factory.example.com/image/abc/v1.13.0/metal-amd64.iso',
    },
  ],
  extraAssets: [],
  talosctl: [],
  installerImage: 'factory.example.com/metal-installer/abc:v1.13.0',
}

/** Answers the API requests that the wizard makes, and records them. */
function stubAPI() {
  const requests: { method: string; url: URL; body: string }[] = []

  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
    const request = input instanceof Request ? input : new Request(input, init)
    const url = new URL(request.url)

    requests.push({ method: request.method, url, body: await request.clone().text() })

    const json = (body: unknown, status = 200) =>
      new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
      })

    switch (`${request.method} ${url.pathname}`) {
      case 'GET /versions':
        return json(['v1.12.0', 'v1.13.0', 'v1.14.0-alpha.0'])
      case 'GET /api/version/1.13.0/features':
        return json(features)
      case 'GET /version/1.13.0/extensions/official':
        return json([
          {
            name: 'siderolabs/gvisor',
            ref: 'ghcr.io/siderolabs/gvisor:20250505.0',
            digest: 'sha256:x',
            author: 'Sidero Labs',
            description: 'gVisor runtime',
          },
          {
            name: 'siderolabs/metal-agent',
            ref: 'ghcr.io/siderolabs/metal-agent:v0.1.0',
            digest: 'sha256:y',
            author: 'Sidero Labs',
            description: 'internal',
          },
        ])
      case 'POST /schematics':
        return json({ id: 'abc', schematic: schematicYAML }, 201)
      case 'GET /api/schematics/abc/1.13.0/assets':
        return json(assets)
      default:
        return json('not found', 404)
    }
  })

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
  afterEach(() => vi.restoreAllMocks())

  test('builds a bare-metal schematic and links its downloads', async () => {
    const requests = stubAPI()
    const { router, screen } = await renderApp('/')
    const next = () => screen.getByRole('button', { name: 'Next' }).click()

    await expect.element(screen.getByRole('heading', { name: 'Hardware Type' })).toBeVisible()
    await next()

    // The latest stable version is picked, not the pre-release.
    await expect
      .element(screen.getByRole('combobox', { name: 'Choose Talos Linux Version' }))
      .toHaveValue('1.13.0')
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
    await expect.element(screen.getByText(assets.bootAssets[0].url, { exact: true })).toBeVisible()
    await expect.element(screen.getByText(assets.installerImage).first()).toBeVisible()

    expect(router.currentRoute.value.query).toEqual({
      target: 'metal',
      version: '1.13.0',
      arch: 'amd64',
      platform: 'metal',
      'schematic-id': 'abc',
    })

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
    stubAPI()
    const { router, screen } = await renderApp('/?target=metal&version=1.13.0&platform=metal')

    await expect
      .element(screen.getByRole('heading', { name: 'Machine Architecture' }))
      .toBeVisible()
    await screen.getByRole('button', { name: 'Back' }).click()

    await expect
      .element(screen.getByRole('combobox', { name: 'Choose Talos Linux Version' }))
      .toHaveValue('1.13.0')
    expect(router.currentRoute.value.query).toEqual({ target: 'metal', platform: 'metal' })
  })
})
