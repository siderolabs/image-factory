<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
defineProps<{
  message: string
}>()
</script>

<template>
  <p role="alert" class="status-glyph status-failed mb-compact type-body text-content-default">
    {{ message }}
  </p>
</template>
