<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { Translation, useI18n } from 'vue-i18n'

import { config } from '@/config'

const { t } = useI18n()
</script>

<template>
  <div class="mt-compact max-w-xl leading-[1.7142857] text-content-default">
    <p class="mb-compact">
      <Translation scope="global" keypath="info.intro">
        <template #company>
          <!-- prettier-ignore -->
          <a href="https://siderolabs.com/" class="font-medium text-accent-text underline">Sidero Labs, Inc.</a>
        </template>
        <template #distro>
          <!-- prettier-ignore -->
          <a href="https://talos.dev/" class="font-medium text-accent-text underline">Talos Linux</a>
        </template>
      </Translation>
    </p>
    <p class="mb-compact">
      <Translation scope="global" keypath="info.more_text">
        <template #github>
          <a
            href="https://github.com/siderolabs/image-factory"
            class="font-medium text-accent-text underline"
          >
            {{ t('info.github') }}
          </a>
        </template>
      </Translation>
    </p>
    <p class="type-meta text-content-secondary">
      {{ t('info.version', { version: config.version }) }}
    </p>
  </div>
</template>
