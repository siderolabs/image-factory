<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { watchEffect } from 'vue'

import AppHeader from '@/components/AppHeader.vue'
import { useTheme } from '@/lib/theme'

const { theme, applied } = useTheme()

// /js/theme.js applies the stored theme before first paint; this keeps <html> in step with later changes.
watchEffect(() => {
  const root = document.documentElement

  root.dataset.choice = theme.value
  root.dataset.theme = applied.value
  root.style.colorScheme = applied.value
})
</script>

<template>
  <AppHeader />
  <RouterView />
</template>
