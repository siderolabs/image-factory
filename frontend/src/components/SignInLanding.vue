<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { config } from '@/config'

const { t } = useI18n()
</script>

<template>
  <main class="container mx-auto max-w-3xl px-base py-section">
    <p class="mb-base type-body text-content-default">
      {{ t(config.enterprise ? 'page.description.enterprise' : 'page.description') }}
    </p>
    <!-- A plain link: /login hands the browser over to the identity provider. -->
    <a
      href="/login"
      class="inline-flex items-center rounded-lg bg-accent-fill px-compact py-tight text-center type-control text-content-on-accent transition-colors hover:bg-accent-fill-hover active:bg-accent-fill-active"
    >
      {{ t('auth.sign_in') }}
    </a>
  </main>
</template>
