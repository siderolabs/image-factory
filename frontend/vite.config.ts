// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { existsSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import Vue from '@vitejs/plugin-vue'
import { playwright } from '@vitest/browser-playwright'
import { defineConfig } from 'vite'
import { configDefaults } from 'vitest/config'
import VueRouter from 'vue-router/vite'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [VueRouter(), Vue(), tailwindcss()],
  // Favicons and other static files are served by the Go backend.
  publicDir: false,
  build: {
    rolldownOptions: {
      output: {
        // Everything except index.html is served by the backend from /assets/.
        // Go's //go:embed skips files starting with '_', and rolldown chunks may begin with '_',
        // so prefix those names to avoid the embed exclusion.
        chunkFileNames: 'assets/o[name]-[hash].js',
        assetFileNames: 'assets/o[name]-[hash][extname]',
        entryFileNames: 'assets/o[name]-[hash].js',
      },
    },
  },
  // Optimize these up front: discovering them mid-run reloads the page, and a browser-mode test then
  // ends up with two copies of Vue.
  optimizeDeps: {
    include: ['vue', 'vue-router', 'vitest-browser-vue'],
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    exclude: [...configDefaults.exclude, 'e2e/**'],
    root: fileURLToPath(new URL('./', import.meta.url)),
    browser: {
      enabled: true,
      // Playwright's own browser builds don't run on Alpine (musl), so in the kres js stage use the
      // Chromium that .kres.yaml installs there. Everywhere else, Playwright's own Chromium is used.
      provider: playwright({
        launchOptions: {
          executablePath: existsSync('/etc/alpine-release') ? '/usr/bin/chromium' : undefined,
        },
      }),
      instances: [{ browser: 'chromium' }],
    },
  },
  server: {
    port: 8121,
    // Set HOST env to "true" or 0.0.0.0 to listen on all interfaces
    host: process.env.HOST === 'true' || process.env.HOST || '127.0.0.1',
    // Image Factory proxies to the dev server, from a container when run with docker compose.
    allowedHosts: ['host.docker.internal'],
  },
})
