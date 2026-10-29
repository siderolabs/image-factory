// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { usePreferredDark, useStorage } from '@vueuse/core'
import { computed } from 'vue'

import {
  parseThemeChoice,
  resolveTheme,
  THEME_STORAGE_KEY,
  type ThemeChoice,
} from '@/lib/theme-core'

export { type ThemeChoice, themeChoices } from '@/lib/theme-core'

// Shared with src/theme-init.ts, which applies the stored choice before first paint.
const stored = useStorage<string>(THEME_STORAGE_KEY, 'system', undefined, { writeDefaults: false })
const prefersDark = usePreferredDark()

const theme = computed(() => parseThemeChoice(stored.value))

/** The theme in use: the choice, with system resolved to light or dark. */
const applied = computed(() => resolveTheme(theme.value, prefersDark.value))

/** The theme choice, stored in the browser. */
export function useTheme() {
  return { theme, applied, setTheme: (value: ThemeChoice) => (stored.value = value) }
}
