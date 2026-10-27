<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useTemplateRef } from 'vue'

const model = defineModel<string>()

const { dense } = defineProps<{
  /** Uses less padding, such as for the extensions search box. */
  dense?: boolean
}>()

const input = useTemplateRef('input')

defineExpose({ focus: () => input.value?.focus() })
</script>

<template>
  <input
    ref="input"
    v-model="model"
    class="block w-full appearance-none rounded-lg border border-border-default bg-surface-inset type-body text-content-emphasis placeholder:text-content-muted focus-visible:border-border-focus"
    :class="dense ? 'p-tight' : 'p-snug'"
  />
</template>
