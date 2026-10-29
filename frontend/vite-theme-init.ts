// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { fileURLToPath, URL } from 'node:url'

import { build, type Plugin } from 'vite'

const srcDir = fileURLToPath(new URL('./src', import.meta.url))

/**
 * Inlines src/theme-init.ts as a classic script in <head>, so the theme is set before first paint.
 */
export function themeInit(): Plugin {
  return {
    name: 'theme-init',
    async transformIndexHtml() {
      const result = await build({
        configFile: false,
        logLevel: 'silent',
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

      return [{ tag: 'script', children: output.output[0].code, injectTo: 'head-prepend' }]
    },
  }
}
