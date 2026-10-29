// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { describe, expect, test } from 'vitest'

import { readConfig } from '@/config'

function page(script?: string) {
  const root = document.createElement('div')

  if (script !== undefined) {
    root.innerHTML = `<script id="app-config" type="application/json">${script}</script>`
  }

  return root
}

describe('readConfig', () => {
  test('reads the configuration filled in by the backend', () => {
    expect(
      readConfig(
        page(
          '{"version":"v1.8.0","enterprise":true,"tokensEnabled":true,"logoutEnabled":true,"authenticated":true}',
        ),
      ),
    ).toEqual({
      version: 'v1.8.0',
      enterprise: true,
      tokensEnabled: true,
      logoutEnabled: true,
      authenticated: true,
    })
  })

  test('falls back to the defaults without a backend', () => {
    const defaults = {
      version: '',
      enterprise: false,
      tokensEnabled: false,
      logoutEnabled: false,
      authenticated: false,
    }

    expect(readConfig(page())).toEqual(defaults)
    expect(readConfig(page('{{ .Config }}'))).toEqual(defaults)
  })
})
