<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { Translation, useI18n } from 'vue-i18n'

import { featuresQuery, platformsQuery } from '@/api/queries'
import AppSpinner from '@/components/AppSpinner.vue'
import ErrorNote from '@/components/ErrorNote.vue'
import RadioCard from '@/components/RadioCard.vue'
import WizardFooter from '@/components/WizardFooter.vue'
import { docsURL } from '@/lib/docs'
import { draft } from '@/wizard/draft'
import type { Arch, Target, WizardParams } from '@/wizard/params'

const { target, version, platform } = defineProps<{
  target: Target
  version: string
  platform?: string
}>()

const emit = defineEmits<{
  back: []
  next: [patch: Partial<WizardParams>]
}>()

const { t } = useI18n()

const {
  data: features,
  isLoading: featuresLoading,
  error: featuresError,
} = useQuery(() => featuresQuery(version))
const {
  data: platforms,
  isLoading: platformsLoading,
  error: platformsError,
} = useQuery(() => ({ ...platformsQuery(version), enabled: target === 'cloud' }))

const arch = ref<Arch>(draft.value.arch ?? 'amd64')
const secureBoot = ref(draft.value.secureBoot ?? false)

const loading = computed(() => featuresLoading.value || platformsLoading.value)
const error = computed(() => (featuresError.value ?? platformsError.value)?.message)

// SecureBoot needs a version that supports it, and a platform that does: bare metal always does.
const secureBootSupported = computed(() => {
  if (!features.value?.includes('SecureBoot')) return false
  if (target === 'metal') return true

  return !!platforms.value?.find((p) => p.name === platform)?.secureBootSupported
})

function next() {
  emit('next', { arch: arch.value, secureBoot: secureBootSupported.value && secureBoot.value })
}
</script>

<template>
  <form @submit.prevent="next">
    <h1 class="mb-snug type-section-title text-content-emphasis">{{ t('architecture.title') }}</h1>
    <AppSpinner v-if="loading" class="size-8" />
    <ErrorNote v-else-if="error" :message="error" />
    <div v-else class="mb-base space-y-tight">
      <RadioCard v-model="arch" name="arch" value="amd64" label="amd64" mono>
        {{ t('architecture.amd64') }}
      </RadioCard>
      <RadioCard v-model="arch" name="arch" value="arm64" label="arm64" mono>
        {{ t(target === 'metal' ? 'architecture.arm64' : 'architecture.other') }}
      </RadioCard>
      <div v-if="secureBootSupported" class="pt-section">
        <label class="inline-flex cursor-pointer items-center">
          <input v-model="secureBoot" type="checkbox" name="secureboot" class="peer sr-only" />
          <div
            class="relative h-6 w-11 rounded-full bg-border-control peer-checked:bg-accent-fill peer-focus-visible:shadow-focus after:absolute after:inset-s-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-content-on-accent after:transition-all after:content-[''] peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full"
          ></div>
          <span class="ms-snug type-control text-content-emphasis">SecureBoot</span>
        </label>
        <div class="ms-tight type-body">
          <p class="type-meta text-content-secondary">
            <Translation scope="global" keypath="architecture.secure_boot.text">
              <template #link>
                <!-- prettier-ignore -->
                <a :href="docsURL(version, '/platform-specific-installations/bare-metal-platforms/secureboot/')" target="_blank" rel="noopener noreferrer" class="text-accent-text underline">SecureBoot</a>
              </template>
            </Translation>
          </p>
        </div>
      </div>
    </div>
    <WizardFooter back :next="!loading && !error" @back="emit('back')" />
  </form>
</template>
