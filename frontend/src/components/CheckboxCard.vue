<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useId } from 'vue'

const model = defineModel<string[]>({ required: true })

defineProps<{
  value: string
  label: string
  description: string
}>()

const id = useId()
</script>

<template>
  <label
    :for="id"
    class="mb-tight flex cursor-pointer items-start gap-snug rounded-lg border border-border-default p-snug transition-colors hover:border-border-strong hover:bg-surface-hover has-checked:border-border-accent has-checked:bg-accent-subtle"
  >
    <input
      :id="id"
      v-model="model"
      :aria-describedby="`${id}-text`"
      type="checkbox"
      :value="value"
      class="mt-micro h-3.5 w-3.5 shrink-0 appearance-none rounded border border-border-control bg-transparent bg-size-[100%_100%] bg-center bg-no-repeat bg-origin-border align-middle select-none checked:border-accent-fill checked:bg-accent-fill"
    />
    <div class="type-body">
      <div class="type-control text-content-emphasis">{{ label }}</div>
      <p :id="`${id}-text`" class="mt-micro type-meta text-content-secondary">
        {{ description }}
      </p>
    </div>
  </label>
</template>

<style scoped>
/* The tick of a checked checkbox. */
input:checked {
  background-image: url("data:image/svg+xml,%3csvg viewBox='0 0 16 16' fill='white' xmlns='http://www.w3.org/2000/svg'%3e%3cpath d='M12.207 4.793a1 1 0 010 1.414l-5 5a1 1 0 01-1.414 0l-2-2a1 1 0 011.414-1.414L6.5 9.086l4.293-4.293a1 1 0 011.414 0z'/%3e%3c/svg%3e");
}

@media (forced-colors: active) {
  input:checked {
    appearance: auto;
  }
}
</style>
