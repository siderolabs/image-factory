<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useId } from 'vue'
import { useI18n } from 'vue-i18n'

import AppSelect from '@/components/AppSelect.vue'
import { type Language, languageNames, languages, setLanguage } from '@/i18n'

const { t, locale } = useI18n()
const id = useId()
</script>

<template>
  <div
    class="mt-base mb-base flex items-center justify-end gap-tight type-label text-content-secondary"
  >
    <label :for="id">{{ t('lang.name') }}</label>
    <AppSelect
      :id="id"
      :model-value="locale"
      class="w-28"
      dense
      @update:model-value="setLanguage($event as Language)"
    >
      <option v-for="lang in languages" :key="lang" :value="lang">{{ languageNames[lang] }}</option>
    </AppSelect>
  </div>
</template>
