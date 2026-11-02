<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'

import AppIcon from '@/components/AppIcon.vue'

const { text } = defineProps<{
  text: string
}>()

// legacy falls back to execCommand on plain http origins other than localhost, which have no Clipboard API.
const { copy, copied } = useClipboard({ legacy: true, copiedDuring: 1000 })
</script>

<template>
  <button
    type="button"
    class="group relative size-6 shrink-0 rounded-sm"
    title="Copy to clipboard"
    aria-label="Copy to clipboard"
    @click="copy(text)"
  >
    <AppIcon
      name="check"
      class="absolute inset-0 m-auto size-4 text-status-success-text transition-all duration-300"
      :class="copied ? 'opacity-100' : 'opacity-0'"
    />
    <AppIcon
      name="copy"
      class="absolute inset-0 m-auto size-4 text-content-secondary transition-all duration-300 group-hover:text-content-emphasis"
      :class="copied ? 'opacity-0' : 'opacity-100'"
    />
  </button>
</template>
