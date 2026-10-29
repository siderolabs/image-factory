<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useTitle } from '@vueuse/core'
import { watchEffect } from 'vue'
import { useI18n } from 'vue-i18n'

import AppHeader from '@/components/AppHeader.vue'
import { config } from '@/config'
import { useTheme } from '@/lib/theme'
import { applyTheme } from '@/lib/theme-core'

const { t } = useI18n()
const { theme, applied } = useTheme()

useTitle(() => t(config.enterprise ? 'page.header.enterprise' : 'page.header'))

// src/theme-init.ts applies the stored theme before first paint; this keeps <html> in step with later changes.
watchEffect(() => applyTheme(theme.value, applied.value))
</script>

<template>
  <AppHeader />
  <RouterView />
</template>
