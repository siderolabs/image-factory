// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Applies the theme before first paint. Inlined into <head> by vite-theme-init.ts, as module scripts run too late.

import { applyTheme, parseThemeChoice, resolveTheme, THEME_STORAGE_KEY } from '@/lib/theme-core'

const media = window.matchMedia('(prefers-color-scheme: dark)')

function apply() {
  let stored: string | null = null

  try {
    stored = localStorage.getItem(THEME_STORAGE_KEY)
  } catch {
    // Blocked storage: follow the system.
  }

  const choice = parseThemeChoice(stored)

  applyTheme(choice, resolveTheme(choice, media.matches))
}

// Also follows system changes on pages without the app, such as the Enterprise sign-in pages.
media.addEventListener('change', apply)
apply()
