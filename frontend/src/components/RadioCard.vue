<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useId } from 'vue'

const model = defineModel<string>()

const { name, value, label, mono } = defineProps<{
  name: string
  value: string
  label: string
  /** Shows the label in a monospace font, for identifiers such as amd64. */
  mono?: boolean
}>()

const id = useId()
</script>

<template>
  <label
    :for="id"
    class="flex cursor-pointer items-start gap-snug rounded-lg border border-border-default p-snug transition-colors hover:border-border-strong hover:bg-surface-hover has-checked:border-border-accent has-checked:bg-accent-subtle"
  >
    <input
      :id="id"
      v-model="model"
      :aria-describedby="`${id}-text`"
      type="radio"
      :name="name"
      :value="value"
      class="mt-micro h-3.5 w-3.5 shrink-0 appearance-none rounded-full border border-border-control bg-transparent bg-size-[100%_100%] bg-center bg-no-repeat bg-origin-border align-middle select-none checked:border-accent-fill checked:bg-accent-fill"
    />
    <div class="type-body">
      <div class="text-content-emphasis" :class="mono ? 'type-mono' : 'type-control'">
        {{ label }}
      </div>
      <p :id="`${id}-text`" class="mt-micro type-meta text-content-secondary">
        <slot />
      </p>
    </div>
  </label>
</template>

<style scoped>
/* The dot of a checked radio button. */
input:checked {
  background-image: url("data:image/svg+xml,%3csvg viewBox='0 0 16 16' fill='white' xmlns='http://www.w3.org/2000/svg'%3e%3ccircle cx='8' cy='8' r='3'/%3e%3c/svg%3e");
}

@media (forced-colors: active) {
  input:checked {
    appearance: auto;
  }
}
</style>
