// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Applies the theme before first paint. Inlined into <head> by vite-theme-init.ts, as module scripts run too late.

import { applyTheme, resolveTheme, THEME_STORAGE_KEY } from '@/lib/theme-core'

let preference: string | null = null

try {
  preference = localStorage.getItem(THEME_STORAGE_KEY)
} catch {
  // Blocked storage: follow the system.
}

applyTheme(resolveTheme(preference, window.matchMedia('(prefers-color-scheme: dark)').matches))
