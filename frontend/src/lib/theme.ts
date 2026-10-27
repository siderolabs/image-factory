// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { usePreferredDark, useStorage } from '@vueuse/core'
import { computed } from 'vue'

/** The theme choices; system follows the operating system's light or dark preference. */
export const themeChoices = ['light', 'dark', 'system'] as const

export type ThemeChoice = (typeof themeChoices)[number]

// Shared with /js/theme.js, which applies the stored choice before first paint.
const stored = useStorage<string>('theme', 'system', undefined, { writeDefaults: false })
const prefersDark = usePreferredDark()

const theme = computed<ThemeChoice>(() =>
  stored.value === 'light' || stored.value === 'dark' ? stored.value : 'system',
)

/** The theme in use: the choice, with system resolved to light or dark. */
const applied = computed(() =>
  theme.value === 'system' ? (prefersDark.value ? 'dark' : 'light') : theme.value,
)

/** The theme choice, stored in the browser. */
export function useTheme() {
  return { theme, applied, setTheme: (value: ThemeChoice) => (stored.value = value) }
}
