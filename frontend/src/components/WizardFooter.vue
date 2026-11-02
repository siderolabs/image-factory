<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import AppIcon from '@/components/AppIcon.vue'

defineProps<{
  /** Shows the Back button. */
  back?: boolean
  /** Shows the Next button. */
  next?: boolean
  /** Disables the buttons while a step's request is in flight. */
  busy?: boolean
}>()

const emit = defineEmits<{
  back: []
  next: []
}>()

const { t } = useI18n()
</script>

<template>
  <div
    class="flex flex-col items-center border-t border-border-default bg-surface-chrome px-base py-compact"
  >
    <div class="container flex items-center justify-end gap-snug">
      <button
        v-if="back"
        type="button"
        :disabled="busy"
        class="inline-flex items-center gap-tight rounded-lg border border-border-default bg-surface-card px-compact py-tight text-center type-control text-content-emphasis transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-50"
        @click="emit('back')"
      >
        <AppIcon name="arrow" class="size-3.5 rotate-180" />
        {{ t('button.back') }}
      </button>

      <button
        v-if="next"
        type="button"
        :disabled="busy"
        class="inline-flex items-center gap-tight rounded-lg border border-accent-fill bg-accent-fill px-compact py-tight text-center type-control text-content-on-accent transition-colors hover:bg-accent-fill-hover active:bg-accent-fill-active disabled:cursor-not-allowed disabled:opacity-50"
        @click="emit('next')"
      >
        {{ t('button.next') }}
        <AppIcon name="arrow" class="size-3.5" />
      </button>
    </div>
  </div>
</template>
