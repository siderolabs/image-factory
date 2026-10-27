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

// Next submits the step's form, so only Back is an event.
const emit = defineEmits<{
  back: []
}>()

const { t } = useI18n()
</script>

<template>
  <div
    class="sticky bottom-0 z-10 -mx-base mt-base flex gap-snug border-t border-border-default bg-surface-chrome px-base py-compact"
  >
    <button
      v-if="back"
      type="button"
      :disabled="busy"
      class="inline-flex items-center rounded-lg border border-border-default bg-surface-card px-compact py-tight text-center type-control text-content-emphasis transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-50"
      @click="emit('back')"
    >
      <AppIcon name="arrow" class="me-tight h-3.5 w-3.5 rotate-180" />
      {{ t('button.back') }}
    </button>
    <button
      v-if="next"
      type="submit"
      :disabled="busy"
      class="inline-flex items-center rounded-lg bg-accent-fill px-compact py-tight text-center type-control text-content-on-accent transition-colors hover:bg-accent-fill-hover active:bg-accent-fill-active disabled:cursor-not-allowed disabled:opacity-50"
    >
      {{ t('button.next') }}
      <AppIcon name="arrow" class="ms-tight h-3.5 w-3.5" />
    </button>
  </div>
</template>
