<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { ref } from 'vue'
import { Translation, useI18n } from 'vue-i18n'

import RadioOption from '@/components/RadioOption.vue'
import { config } from '@/config'
import { draft } from '@/wizard/draft'
import type { Target, WizardParams } from '@/wizard/params'

const emit = defineEmits<{
  next: [patch: Partial<WizardParams>]
}>()

const { t } = useI18n()

const target = ref<Target>(draft.value.target ?? 'metal')

defineExpose({ canNext: true, next: () => emit('next', { target: target.value }) })
</script>

<template>
  <div>
    <div class="mb-section flex flex-col gap-compact leading-relaxed text-content-default">
      <p>
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

      <p>
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

    <h1 class="mb-snug type-section-title text-content-emphasis">{{ t('hardware.title') }}</h1>
    <div class="space-y-snug">
      <RadioOption v-model="target" name="target" value="metal" :label="t('hardware.bmm.name')">
        {{ t('hardware.bmm.description') }}
      </RadioOption>
      <RadioOption v-model="target" name="target" value="cloud" :label="t('hardware.cs.name')">
        {{ t('hardware.cs.description') }}
      </RadioOption>
      <RadioOption v-model="target" name="target" value="sbc" :label="t('hardware.sbc.name')">
        {{ t('hardware.sbc.description') }}
      </RadioOption>
    </div>
  </div>
</template>
