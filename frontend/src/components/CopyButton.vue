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
const { copy, copied } = useClipboard({ legacy: true, copiedDuring: 2000 })
</script>

<template>
  <button
    type="button"
    class="shrink-0 rounded border p-micro transition-colors hover:bg-surface-hover hover:text-content-emphasis"
    :class="
      copied
        ? 'border-border-strong text-content-emphasis'
        : 'border-border-default text-content-default'
    "
    title="Copy to clipboard"
    aria-label="Copy to clipboard"
    @click="copy(text)"
  >
    <AppIcon name="copy" class="h-3 w-3" />
  </button>
</template>
