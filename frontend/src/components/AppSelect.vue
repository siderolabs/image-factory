<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
const model = defineModel<string>()

const { dense } = defineProps<{
  /** Uses less padding, for selects outside forms such as the language picker. */
  dense?: boolean
}>()
</script>

<template>
  <select
    v-model="model"
    class="block appearance-none rounded-lg border border-border-default bg-surface-inset type-body focus-visible:border-border-focus"
    :class="dense ? 'p-tight' : 'p-snug'"
  >
    <slot />
  </select>
</template>

<style scoped>
/* The dropdown arrow. */
select {
  background-image: url("data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 20 20'%3e%3cpath stroke='%236b7280' stroke-linecap='round' stroke-linejoin='round' stroke-width='1.5' d='M6 8l4 4 4-4'/%3e%3c/svg%3e");
  background-position: right 0.5rem center;
  background-repeat: no-repeat;
  background-size: 1.5em 1.5em;
}
</style>
