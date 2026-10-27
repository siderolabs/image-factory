<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { platformsQuery } from '@/api/queries'
import AppSpinner from '@/components/AppSpinner.vue'
import ErrorNote from '@/components/ErrorNote.vue'
import RadioCard from '@/components/RadioCard.vue'
import WizardFooter from '@/components/WizardFooter.vue'
import { docsURL } from '@/lib/docs'
import { draft } from '@/wizard/draft'
import type { Arch, WizardParams } from '@/wizard/params'

const { version } = defineProps<{
  version: string
}>()

const emit = defineEmits<{
  back: []
  next: [patch: Partial<WizardParams>]
}>()

const { t } = useI18n()

const { data: platforms, isLoading, error } = useQuery(() => platformsQuery(version))
const platform = ref<string>()

watch(
  platforms,
  (list = []) => {
    const names = list.map((p) => p.name)
    const preferred = [draft.value.platform, 'aws'].find((p) => p && names.includes(p))

    platform.value = preferred ?? names[0]
  },
  { immediate: true },
)

function next() {
  const chosen = platforms.value?.find((p) => p.name === platform.value)

  if (!chosen) return

  // A platform with a single architecture decides it, and the architecture step is skipped.
  const arch = chosen.architectures.length === 1 ? (chosen.architectures[0] as Arch) : undefined

  emit('next', { platform: chosen.name, arch })
}
</script>

<template>
  <form @submit.prevent="next">
    <h1 class="mb-snug type-section-title text-content-emphasis">{{ t('cloud.title') }}</h1>
    <AppSpinner v-if="isLoading" class="size-8" />
    <ErrorNote v-else-if="error" :message="error.message" />
    <div v-else class="mb-base space-y-tight">
      <RadioCard
        v-for="p in platforms"
        :key="p.name"
        v-model="platform"
        name="platform"
        :value="p.name"
        :label="p.label"
      >
        <!-- prettier-ignore -->
        <span>{{ p.description }} (<a :href="docsURL(version, p.documentation)" target="_blank" rel="noopener noreferrer" class="text-accent-text underline">{{ t('common.documentation') }}</a>).</span>
      </RadioCard>
    </div>
    <WizardFooter back :next="!!platform" @back="emit('back')" />
  </form>
</template>
