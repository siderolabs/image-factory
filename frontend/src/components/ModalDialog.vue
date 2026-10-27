<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { onKeyStroke, useScrollLock } from '@vueuse/core'

const { width = 'md', locked } = defineProps<{
  title: string
  width?: 'md' | 'lg'
  /** Keeps the dialog open while a request it started is in flight. */
  locked?: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

function close() {
  if (!locked) emit('close')
}

onKeyStroke('Escape', close)

// The page behind the dialog doesn't scroll while it's open.
useScrollLock(document.body, true)
</script>

<template>
  <div
    class="fixed inset-0 z-50 flex justify-center overflow-y-auto bg-surface-scrim p-compact"
    @click.self="close"
  >
    <div
      role="dialog"
      aria-modal="true"
      :aria-label="title"
      class="relative my-auto w-full rounded-md border border-border-default bg-surface-card p-base shadow-modal"
      :class="width === 'lg' ? 'max-w-lg' : 'max-w-md'"
    >
      <h2 class="mb-compact type-section-title text-content-emphasis">{{ title }}</h2>
      <slot />
    </div>
  </div>
</template>
