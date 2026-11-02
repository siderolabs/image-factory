// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// No imports: this is inlined into index.html via src/theme-init.ts.

export type ThemePreference = 'light' | 'dark' | 'dim' | 'system'
export type Theme = Exclude<ThemePreference, 'system'>

export const THEME_STORAGE_KEY = 'theme'

/** Dim counts as dark. */
const colorScheme = (theme: Theme): 'light' | 'dark' => (theme === 'light' ? 'light' : 'dark')

const isTheme = (value: unknown): value is Theme =>
  value === 'light' || value === 'dark' || value === 'dim'

/** Unrecognised values follow the system. */
export const resolveTheme = (preference: unknown, prefersDark: boolean): Theme => {
  if (isTheme(preference)) return preference

  return prefersDark ? 'dark' : 'light'
}

/**
 * Sets the theme on <html>, so teleported content follows it. The background is
 * set inline so it paints before the stylesheet loads.
 */
export function applyTheme(theme: Theme) {
  const root = document.documentElement
  const colors = __THEME_COLORS__[theme]

  root.dataset.theme = theme
  root.style.colorScheme = colorScheme(theme)
  root.style.backgroundColor = colors.page

  document
    .querySelector<HTMLMetaElement>('meta[name="theme-color"]')
    ?.setAttribute('content', colors.chrome)
}
