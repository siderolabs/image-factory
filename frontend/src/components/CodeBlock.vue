<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import CopyButton from '@/components/CopyButton.vue'

const { code, block } = defineProps<{
  code: string
  /** Shows multi-line text, such as a configuration patch, as a block. */
  block?: boolean
}>()
</script>

<template>
  <div
    class="gap-tight bg-surface-inset"
    :class="block ? 'flex items-start p-tight' : 'inline-flex items-center p-micro'"
  >
    <span
      class="type-mono"
      :class="{ 'break-all whitespace-pre-wrap text-content-emphasis': block }"
    >
      {{ code }}
    </span>
    <CopyButton :text="code" />
  </div>
</template>
