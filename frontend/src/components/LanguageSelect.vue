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
import { type Language, languageNames, languages, setLanguage } from '@/i18n'

const { t, locale } = useI18n()

const open = ref(false)
const menu = useTemplateRef('menu')

function choose(lang: Language) {
  setLanguage(lang)
  open.value = false
}

onClickOutside(menu, () => (open.value = false))
onKeyStroke('Escape', () => (open.value = false))
</script>

<template>
  <div ref="menu" class="relative">
    <button
      type="button"
      class="flex items-center rounded-sm p-tight text-content-default hover:bg-surface-hover hover:text-content-emphasis"
      :aria-label="t('lang.current', { lang: languageNames[locale as Language] })"
      :title="t('lang.name')"
      :aria-expanded="open"
      @click="open = !open"
    >
      <AppIcon name="language" class="size-4 shrink-0" />
    </button>
    <div
      v-if="open"
      class="absolute inset-e-0 z-20 mt-micro w-40 rounded-lg border border-border-default bg-surface-raised p-micro shadow-dropdown"
    >
      <button
        v-for="lang in languages"
        :key="lang"
        type="button"
        :lang="lang"
        :aria-pressed="locale === lang"
        class="flex w-full items-center gap-tight rounded-sm px-snug py-tight text-left text-content-default hover:bg-surface-hover"
        @click="choose(lang)"
      >
        <span class="grow">{{ languageNames[lang] }}</span>
        <AppIcon name="check" class="size-4 shrink-0" :class="{ invisible: locale !== lang }" />
      </button>
    </div>
  </div>
</template>
