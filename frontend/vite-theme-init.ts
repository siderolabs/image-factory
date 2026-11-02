// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { fileURLToPath, URL } from 'node:url'

import tokens from '@siderolabs/talos-design-system/tokens.json' with { type: 'json' }
import { build, type Plugin } from 'vite'

const srcDir = fileURLToPath(new URL('./src', import.meta.url))

// Only what the pre-paint script needs, not the whole tokens file.
const define = {
  __THEME_COLORS__: JSON.stringify(
    Object.fromEntries(
      (['light', 'dark', 'dim'] as const).map((theme) => [
        theme,
        {
          page: tokens.semantic[theme]['surface-page'].value,
          chrome: tokens.semantic[theme]['surface-chrome'].value,
        },
      ]),
    ),
  ),
}

/**
 * Inlines src/theme-init.ts as a classic script in <head>, so the theme is set
 * before first paint. Also defines `__THEME_COLORS__`.
 */
export function themeInit(): Plugin {
  return {
    name: 'theme-init',
    config: () => ({ define }),
    async transformIndexHtml() {
      const result = await build({
        configFile: false,
        logLevel: 'silent',
        define,
        resolve: { alias: { '@': srcDir } },
        build: {
          write: false,
          copyPublicDir: false,
          rolldownOptions: {
            input: `${srcDir}/theme-init.ts`,
            output: { format: 'iife' },
          },
        },
      })

      const [output] = Array.isArray(result) ? result : [result]

      if (!('output' in output)) throw new Error('theme-init: unexpected build result')

      // At the end of <head>, after the theme-color meta tag that it updates.
      return [{ tag: 'script', children: output.output[0].code, injectTo: 'head' }]
    },
  }
}
