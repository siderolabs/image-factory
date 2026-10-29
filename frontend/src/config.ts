// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

/** Server configuration, filled into index.html by the backend on every request. */
export interface Config {
  /** Image Factory version. */
  version: string
  /** Whether this is an Enterprise build. */
  enterprise: boolean
  /** Whether self-issued API tokens are enabled. */
  tokensEnabled: boolean
  /** Whether users log in through the browser, and so can log out. */
  logoutEnabled: boolean
  /** Whether the page was requested with credentials, such as a browser login session. */
  authenticated: boolean
}

const defaults: Config = {
  version: '',
  enterprise: false,
  tokensEnabled: false,
  logoutEnabled: false,
  authenticated: false,
}

/**
 * Reads the configuration from index.html. Without a backend (tests, or the Vite dev server opened directly),
 * the template is not filled in, and the defaults apply.
 */
export function readConfig(root: ParentNode = document): Config {
  const text = root.querySelector('#app-config')?.textContent

  if (!text) return defaults

  try {
    return { ...defaults, ...JSON.parse(text) }
  } catch {
    return defaults
  }
}

export const config = readConfig()
