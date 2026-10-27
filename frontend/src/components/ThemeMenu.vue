<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { onClickOutside, onKeyStroke } from '@vueuse/core'
import { ref, useTemplateRef } from 'vue'
import { useI18n } from 'vue-i18n'

import AppIcon from '@/components/AppIcon.vue'
import { type ThemeChoice, themeChoices, useTheme } from '@/lib/theme'

const { t } = useI18n()
const { theme, setTheme } = useTheme()

const open = ref(false)
const menu = useTemplateRef('menu')

const icons = { light: 'sun', dark: 'moon', system: 'monitor' } as const

function choose(choice: ThemeChoice) {
  setTheme(choice)
  open.value = false
}

onClickOutside(menu, () => (open.value = false))
onKeyStroke('Escape', () => (open.value = false))
</script>

<template>
  <div ref="menu" class="relative">
    <button
      type="button"
      class="flex items-center rounded-md p-tight text-content-default hover:bg-surface-hover hover:text-content-emphasis"
      :aria-label="t('nav.theme.current', { theme: t(`nav.theme.${theme}`) })"
      :title="t('nav.theme')"
      :aria-expanded="open"
      @click="open = !open"
    >
      <AppIcon :name="icons[theme]" class="size-4 shrink-0" />
    </button>
    <div
      v-if="open"
      class="absolute inset-e-0 z-20 mt-micro w-40 rounded-lg border border-border-default bg-surface-raised p-micro shadow-dropdown"
    >
      <button
        v-for="choice in themeChoices"
        :key="choice"
        type="button"
        :aria-pressed="theme === choice"
        class="flex w-full items-center gap-tight rounded-md px-snug py-tight text-left text-content-default hover:bg-surface-hover"
        @click="choose(choice)"
      >
        <AppIcon :name="icons[choice]" class="size-4 shrink-0" />
        <span class="grow">{{ t(`nav.theme.${choice}`) }}</span>
        <AppIcon name="check" class="size-4 shrink-0" :class="{ invisible: theme !== choice }" />
      </button>
    </div>
  </div>
</template>
