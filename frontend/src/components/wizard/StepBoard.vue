<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { ref, watch } from 'vue'
import { Translation, useI18n } from 'vue-i18n'

import { boardsQuery } from '@/api/queries'
import AppSpinner from '@/components/AppSpinner.vue'
import ErrorNote from '@/components/ErrorNote.vue'
import RadioCard from '@/components/RadioCard.vue'
import WizardFooter from '@/components/WizardFooter.vue'
import { docsURL } from '@/lib/docs'
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

const { data: boards, isLoading, error } = useQuery(() => boardsQuery(version))
const board = ref<string>()

watch(
  boards,
  (list = []) => {
    const names = list.map((b) => b.name)
    const preferred = [draft.value.board, 'rpi_generic'].find((b) => b && names.includes(b))

    board.value = preferred ?? names[0]
  },
  { immediate: true },
)
</script>

<template>
  <!-- Single-board computers are arm64, so the board decides the architecture. -->
  <form @submit.prevent="board && emit('next', { board, arch: 'arm64' })">
    <h1 class="mb-snug type-section-title text-content-emphasis">{{ t('singleboard.title') }}</h1>
    <AppSpinner v-if="isLoading" class="size-8" />
    <ErrorNote v-else-if="error" :message="error.message" />
    <div v-else class="mb-base space-y-tight">
      <RadioCard
        v-for="b in boards"
        :key="b.name"
        v-model="board"
        name="board"
        :value="b.name"
        :label="b.label"
      >
        <Translation v-if="b.documentation" scope="global" keypath="common.runs_on_board_with_docs">
          <template #board>{{ b.label }}</template>
          <template #link>
            <a
              :href="docsURL(version, b.documentation)"
              target="_blank"
              rel="noopener noreferrer"
              class="text-accent-text underline"
            >
              {{ t('common.documentation') }}
            </a>
          </template>
        </Translation>
        <template v-else>{{ t('common.runs_on_board', { board: b.label }) }}</template>
      </RadioCard>
    </div>
    <WizardFooter back :next="!!board" @back="emit('back')" />
  </form>
</template>
