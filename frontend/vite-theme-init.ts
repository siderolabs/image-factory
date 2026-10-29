// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { fileURLToPath, URL } from 'node:url'

import { build, type Plugin } from 'vite'

const srcDir = fileURLToPath(new URL('./src', import.meta.url))

/** Where the script is also served from, for the Enterprise sign-in pages, which are rendered outside the app. */
const scriptPath = 'js/theme.js'

async function compile() {
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

  return output.output[0].code
}

/**
 * Inlines src/theme-init.ts as a classic script in <head>, so the theme is set before first paint. The same script
 * is also served at /js/theme.js.
 */
export function themeInit(): Plugin {
  return {
    name: 'theme-init',
    async transformIndexHtml() {
      return [{ tag: 'script', children: await compile(), injectTo: 'head-prepend' }]
    },
    async generateBundle() {
      this.emitFile({ type: 'asset', fileName: scriptPath, source: await compile() })
    },
    configureServer(server) {
      server.middlewares.use(`/${scriptPath}`, (_, res, next) => {
        compile()
          .then((code) => {
            res.setHeader('Content-Type', 'text/javascript')
            res.end(code)
          })
          .catch(next)
      })
    },
  }
}
