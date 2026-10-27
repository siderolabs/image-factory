<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { Translation, useI18n } from 'vue-i18n'

import { extensionsQuery } from '@/api/queries'
import AppIcon from '@/components/AppIcon.vue'
import AppInput from '@/components/AppInput.vue'
import AppSpinner from '@/components/AppSpinner.vue'
import CheckboxCard from '@/components/CheckboxCard.vue'
import ErrorNote from '@/components/ErrorNote.vue'
import WizardFooter from '@/components/WizardFooter.vue'
import { draft } from '@/wizard/draft'
import type { WizardParams } from '@/wizard/params'

const { version } = defineProps<{
  version: string
}>()

const emit = defineEmits<{
  back: []
  next: [patch: Partial<WizardParams>]
}>()

const { t } = useI18n()

const { data: extensions, isLoading, error } = useQuery(() => extensionsQuery(version))
const selected = ref<string[]>([...(draft.value.extensions ?? [])])
const search = ref('')

// The search matches the name, reference and description, and always keeps the selected extensions in view.
const shown = computed(() => {
  const query = search.value.trim().toLowerCase()

  return (extensions.value ?? []).filter(
    (e) =>
      !query ||
      selected.value.includes(e.name) ||
      e.ref.toLowerCase().includes(query) ||
      e.description.toLowerCase().includes(query),
  )
})

function tag(ref: string) {
  return ref.slice(ref.lastIndexOf(':') + 1)
}
</script>

<template>
  <form @submit.prevent="emit('next', { extensions: selected })">
    <h1 class="mb-snug type-section-title text-content-emphasis">{{ t('extensions.title') }}</h1>
    <div class="mb-base flex max-h-125 flex-col">
      <div class="ms-tight type-body">
        <p class="type-meta text-content-secondary">
          <Translation scope="global" keypath="extensions.description_text">
            <template #link>
              <a
                href="https://github.com/siderolabs/extensions"
                target="_blank"
                rel="noopener noreferrer"
                class="text-accent-text underline"
              >
                {{ t('extensions.title') }}
              </a>
            </template>
          </Translation>
          <br />
          {{ t('extensions.select') }}
        </p>
      </div>

      <div class="mt-tight mb-tight ps-tight">
        <div class="relative">
          <div class="pointer-events-none absolute inset-y-0 inset-s-0 flex items-center ps-snug">
            <AppIcon name="search" class="h-4 w-4 text-content-secondary" />
          </div>
          <AppInput
            v-model="search"
            type="search"
            :aria-label="t('button.search')"
            :placeholder="t('button.search')"
            class="ps-section"
            dense
          />
        </div>
      </div>

      <AppSpinner v-if="isLoading" class="size-8" />
      <ErrorNote v-else-if="error" :message="error.message" />
      <div v-else class="min-h-0 flex-1 overflow-y-auto ps-tight">
        <CheckboxCard
          v-for="e in shown"
          :key="e.name"
          v-model="selected"
          :value="e.name"
          :label="`${e.name} (${tag(e.ref)})`"
          :description="e.description"
        />
      </div>
      <WizardFooter back :next="!!extensions" @back="emit('back')" />
    </div>
  </form>
</template>
