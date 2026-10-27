<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import RadioCard from '@/components/RadioCard.vue'
import WizardFooter from '@/components/WizardFooter.vue'
import { draft } from '@/wizard/draft'
import type { Target, WizardParams } from '@/wizard/params'

const emit = defineEmits<{
  next: [patch: Partial<WizardParams>]
}>()

const { t } = useI18n()

const target = ref<Target>(draft.value.target ?? 'metal')
</script>

<template>
  <form @submit.prevent="emit('next', { target })">
    <h1 class="mb-snug type-section-title text-content-emphasis">{{ t('hardware.title') }}</h1>
    <div class="mb-base space-y-tight">
      <RadioCard v-model="target" name="target" value="metal" :label="t('hardware.bmm.name')">
        {{ t('hardware.bmm.description') }}
      </RadioCard>
      <RadioCard v-model="target" name="target" value="cloud" :label="t('hardware.cs.name')">
        {{ t('hardware.cs.description') }}
      </RadioCard>
      <RadioCard v-model="target" name="target" value="sbc" :label="t('hardware.sbc.name')">
        {{ t('hardware.sbc.description') }}
      </RadioCard>
    </div>
    <WizardFooter next />
  </form>
</template>
