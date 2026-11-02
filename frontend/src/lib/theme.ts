// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { createSharedComposable, useLocalStorage, usePreferredDark } from '@vueuse/core'
import { computed } from 'vue'

import { resolveTheme, THEME_STORAGE_KEY, type ThemePreference } from '@/lib/theme-core'

export { applyTheme, type Theme, type ThemePreference } from '@/lib/theme-core'

export const useTheme = createSharedComposable(() => {
  const preference = useLocalStorage<ThemePreference>(THEME_STORAGE_KEY, 'system')
  const prefersDark = usePreferredDark()

  const theme = computed(() => resolveTheme(preference.value, prefersDark.value))

  return { preference, theme }
})
