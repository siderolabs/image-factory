// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import path from 'node:path'

import { defineConfig, devices } from '@playwright/test'
import dotenv from 'dotenv'

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
dotenv.config({ quiet: true })

const permissions = ['clipboard-read', 'clipboard-write']

/**
 * Specs run against the community image factory, except the ones tagged as enterprise-only, which run on their own
 * against an enterprise factory, and the Auth0 browser login ones, which run against an enterprise factory that signs
 * in with Auth0.
 */
const ENTERPRISE_ONLY = /@enterprise-factory/
const AUTH0_ONLY = /@auth0/

const chrome = {
  ...devices['Desktop Chrome'],
}

const firefox = {
  ...devices['Desktop Firefox'],
  // Firefox does not support clipboard permissions
  permissions: permissions.filter((p) => !p.startsWith('clipboard')),
  launchOptions: {
    firefoxUserPrefs: {
      // Firefox specific flag to always allow clipboard access
      'dom.events.testing.asyncClipboard': true,
    },
  },
}

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  testDir: path.join('.', 'e2e'),
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: process.env.CI ? [['html'], ['list'], ['github']] : [['html'], ['list']],
  use: {
    baseURL: process.env.BASE_URL,
    // A factory with authentication enabled uses HTTP Basic authentication in development and tests.
    httpCredentials: process.env.AUTH_USERNAME
      ? { username: process.env.AUTH_USERNAME, password: process.env.AUTH_PASSWORD ?? '' }
      : undefined,
    permissions,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
    ignoreHTTPSErrors: true,
    video: {
      mode: process.env.CI ? 'retain-on-failure' : 'on',
      size: { width: 1280, height: 720 },
    },
  },

  projects: [
    {
      name: 'community-chrome',
      use: chrome,
      grepInvert: [ENTERPRISE_ONLY, AUTH0_ONLY],
    },
    {
      name: 'community-firefox',
      use: firefox,
      grepInvert: [ENTERPRISE_ONLY, AUTH0_ONLY],
    },
    {
      name: 'community',
      testMatch: /(?!)/,
      dependencies: ['community-chrome', 'community-firefox'],
    },
    {
      name: 'enterprise-chrome',
      use: chrome,
      grep: ENTERPRISE_ONLY,
    },
    {
      name: 'enterprise-firefox',
      use: firefox,
      grep: ENTERPRISE_ONLY,
    },
    {
      // The enterprise-only specs, run against an enterprise image factory.
      name: 'enterprise',
      testMatch: /(?!)/,
      dependencies: ['enterprise-chrome', 'enterprise-firefox'],
    },
    {
      name: 'auth0-chrome',
      use: chrome,
      grep: AUTH0_ONLY,
    },
    {
      name: 'auth0-firefox',
      use: firefox,
      grep: AUTH0_ONLY,
    },
    {
      // The Auth0 browser login specs, run against an enterprise image factory that signs in with Auth0.
      name: 'auth0',
      testMatch: /(?!)/,
      dependencies: ['auth0-chrome', 'auth0-firefox'],
    },
  ],
})
