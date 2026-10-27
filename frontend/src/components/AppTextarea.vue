<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
const model = defineModel<string>()
</script>

<template>
  <textarea
    v-model="model"
    class="block w-full rounded-lg border border-border-default bg-surface-inset p-snug type-body text-content-emphasis placeholder:text-content-muted focus-visible:border-border-focus"
  ></textarea>
</template>
