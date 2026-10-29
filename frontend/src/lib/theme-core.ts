// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// No imports: this is inlined into index.html via src/theme-init.ts.

/** The theme choices; system follows the operating system's light or dark preference. */
export const themeChoices = ['light', 'dark', 'system'] as const

export type ThemeChoice = (typeof themeChoices)[number]

export type Theme = Exclude<ThemeChoice, 'system'>

export const THEME_STORAGE_KEY = 'theme'

/** Unrecognised values follow the system. */
export const parseThemeChoice = (value: unknown): ThemeChoice =>
  value === 'light' || value === 'dark' ? value : 'system'

/** The theme a choice shows: the choice itself, or the system's preference. */
export const resolveTheme = (choice: ThemeChoice, prefersDark: boolean): Theme =>
  choice === 'system' ? (prefersDark ? 'dark' : 'light') : choice

/** Sets the theme on <html>, so teleported content follows it. */
export function applyTheme(choice: ThemeChoice, theme: Theme) {
  const root = document.documentElement

  root.dataset.choice = choice
  root.dataset.theme = theme
  root.style.colorScheme = theme
}
