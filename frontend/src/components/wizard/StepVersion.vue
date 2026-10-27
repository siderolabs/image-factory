<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { versionsQuery } from '@/api/queries'
import AppSelect from '@/components/AppSelect.vue'
import AppSpinner from '@/components/AppSpinner.vue'
import ErrorNote from '@/components/ErrorNote.vue'
import WizardFooter from '@/components/WizardFooter.vue'
import { docsURL } from '@/lib/docs'
import { groupVersions } from '@/lib/versions'
import { draft } from '@/wizard/draft'
import type { Target, WizardParams } from '@/wizard/params'

const { target } = defineProps<{
  target: Target
}>()

const emit = defineEmits<{
  back: []
  next: [patch: Partial<WizardParams>]
}>()

const { t } = useI18n()

const { data: versions, isLoading, error } = useQuery(() => versionsQuery(target))
const choices = computed(() => groupVersions(versions.value ?? []))

const version = ref<string>()

// Start from the version picked before if this target offers it, otherwise from the latest stable one.
watch(
  choices,
  ({ groups, latestStable }) => {
    const offered = groups.flatMap((g) => g.versions)

    version.value =
      draft.value.version && offered.includes(draft.value.version)
        ? draft.value.version
        : latestStable
  },
  { immediate: true },
)
</script>

<template>
  <form @submit.prevent="version && emit('next', { version })">
    <h1 class="mb-snug type-section-title text-content-emphasis">{{ t('versions.title') }}</h1>
    <div class="mb-base">
      <div
        v-if="target === 'sbc'"
        class="status-glyph status-info mb-compact rounded-lg border border-border-default bg-surface-card p-compact type-body text-content-default"
        role="note"
      >
        {{ t('versions.sbc_note') }}
      </div>

      <AppSpinner v-if="isLoading" class="size-8" />
      <ErrorNote v-else-if="error" :message="error.message" />
      <template v-else>
        <AppSelect v-model="version" name="version" :aria-label="t('versions.title')" class="w-64">
          <optgroup v-for="group in choices.groups" :key="group.label" :label="group.label">
            <option v-for="v in group.versions" :key="v" :value="v">{{ v }}</option>
          </optgroup>
        </AppSelect>
        <p class="mt-base type-meta text-content-secondary">
          {{ t('versions.recommendation', { version: choices.latestStable }) }}
          <br />
          {{ t('versions.recommendation.pre_release') }}
        </p>
        <div v-if="version" class="mt-base mb-base leading-[1.7142857] text-content-default">
          <p class="mb-compact">{{ t('docs.title', { version }) }}</p>
          <ul class="list-disc ps-base *:my-micro *:ps-micro marker:text-content-secondary">
            <li>
              <a
                target="_blank"
                rel="noopener noreferrer"
                :href="docsURL(version, `/getting-started/what's-new-in-talos/`)"
                class="font-medium text-accent-text underline"
              >
                {{ t('docs.new') }}
              </a>
            </li>
            <li>
              <a
                target="_blank"
                rel="noopener noreferrer"
                :href="docsURL(version, '/getting-started/support-matrix/')"
                class="font-medium text-accent-text underline"
              >
                {{ t('docs.matrix') }}
              </a>
            </li>
          </ul>
        </div>
      </template>
    </div>
    <WizardFooter back :next="!!version" @back="emit('back')" />
  </form>
</template>
