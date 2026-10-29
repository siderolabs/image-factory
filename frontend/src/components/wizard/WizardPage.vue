<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { useRouteQuery } from '@vueuse/router'
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { boardsQuery, createSchematicMutation, platformsQuery, schematicQuery } from '@/api/queries'
import AppSpinner from '@/components/AppSpinner.vue'
import ErrorNote from '@/components/ErrorNote.vue'
import LanguageSelect from '@/components/LanguageSelect.vue'
import PageIntro from '@/components/PageIntro.vue'
import StepArch from '@/components/wizard/StepArch.vue'
import StepBoard from '@/components/wizard/StepBoard.vue'
import StepCustomization from '@/components/wizard/StepCustomization.vue'
import StepExtensions from '@/components/wizard/StepExtensions.vue'
import StepFinal from '@/components/wizard/StepFinal.vue'
import StepPlatform from '@/components/wizard/StepPlatform.vue'
import StepTarget from '@/components/wizard/StepTarget.vue'
import StepVersion from '@/components/wizard/StepVersion.vue'
import WizardFooter from '@/components/WizardFooter.vue'
import { draft } from '@/wizard/draft'
import {
  clearFrom,
  currentStep,
  fromSchematic,
  parseQuery,
  previousStep,
  toQuery,
  toSchematic,
  type WizardParams,
} from '@/wizard/params'

const route = useRoute()
const router = useRouter()

// The language was chosen from ?lang= when the app started, so drop it from the URL.
const lang = useRouteQuery('lang')

onMounted(() => (lang.value = undefined))

const query = computed(() => parseQuery(route.query))

// A schematic link (?schematic-id=) restores the choices that the schematic records.
const {
  data: schematic,
  isLoading: schematicLoading,
  error: schematicError,
} = useQuery(() => ({
  ...schematicQuery(query.value.schematicId ?? ''),
  enabled: !!query.value.schematicId,
}))
const restored = computed(() => (schematic.value ? fromSchematic(schematic.value) : undefined))

// The schematic records a board's overlay, not the board, so a restored board is found by its overlay; and the
// schematic of an SBC records its board's overlay.
const {
  data: boards,
  isLoading: boardsLoading,
  error: boardsError,
} = useQuery(() => ({
  ...boardsQuery(query.value.version ?? ''),
  enabled: !!query.value.version && (query.value.target === 'sbc' || !!restored.value?.overlay),
}))

watch(
  restored,
  (choices) => {
    if (choices) draft.value.embeddedConfig = choices.embeddedConfig
  },
  { immediate: true },
)

const params = computed<WizardParams>(() => {
  const q = query.value
  const choices = restored.value

  if (!choices) return q

  const overlay = choices.overlay
  const board =
    q.board ??
    boards.value?.find(
      (b) => b.overlay.name === overlay?.name && b.overlay.image === overlay?.image,
    )?.name

  return { ...q, ...choices.params, target: q.target ?? choices.params.target, board }
})

const restoring = computed(
  () => schematicLoading.value || (!!restored.value?.overlay && boardsLoading.value),
)
const restoreError = computed(() => (schematicError.value ?? boardsError.value)?.message)

const step = computed(() => currentStep(params.value))

// The platform decides the architecture when it has only one, and Back skips the architecture step then.
const { data: platforms } = useQuery(() => ({
  ...platformsQuery(params.value.version ?? ''),
  enabled: params.value.target === 'cloud' && !!params.value.version,
}))

const archDecided = computed(() => {
  const p = params.value

  if (p.target === 'sbc') return true

  return (
    p.target === 'cloud' &&
    platforms.value?.find((pl) => pl.name === p.platform)?.architectures.length === 1
  )
})

function next(patch: Partial<WizardParams>) {
  Object.assign(draft.value, patch)
  router.push({ query: toQuery({ ...params.value, ...patch, schematicId: undefined }) })
}

function back() {
  const previous = previousStep(step.value, params.value, archDecided.value)

  router.push({ query: previous ? toQuery(clearFrom(previous, params.value)) : {} })
}

// The final step has every choice, including those of a schematic link that the steps never saw, so going back
// from it starts each step from them.
watch(
  () => (step.value === 'final' && params.value.schematicId ? params.value : undefined),
  (choices) => {
    if (choices) Object.assign(draft.value, { ...choices, schematicId: undefined })
  },
  { immediate: true },
)

// A platform or board only applies to its own target, and so do overlay options, as in Omni's installation media
// wizard. The architecture needs no rule: the platform and board steps choose it, or leave it to its own step.
watch(
  () => draft.value.target,
  (target) => {
    if (target !== 'cloud') draft.value.platform = undefined

    if (target !== 'sbc') {
      draft.value.board = undefined
      draft.value.overlayOptions = undefined
    }
  },
)

// The final step creates the schematic of the choices, then links to it.
const {
  mutate: createSchematic,
  reset: resetCreate,
  isPending: createPending,
  error: createError,
} = useMutation(createSchematicMutation())
const creating = computed(
  () => createPending.value || (step.value === 'final' && boardsLoading.value),
)

watch(
  () => {
    const p = params.value

    if (step.value !== 'final' || p.schematicId) return undefined

    // An SBC's schematic needs its board's overlay.
    if (p.target === 'sbc' && !boards.value) return undefined

    return p
  },
  (p) => {
    if (!p) return resetCreate()

    const overlay =
      p.target === 'sbc' ? boards.value?.find((b) => b.name === p.board)?.overlay : undefined

    createSchematic(toSchematic(p, draft.value.embeddedConfig ?? '', overlay), {
      onSuccess: (result) => router.replace({ query: toQuery({ ...p, schematicId: result.id }) }),
    })
  },
  { immediate: true },
)
</script>

<template>
  <main class="container mx-auto max-w-3xl px-base">
    <PageIntro />
    <!-- Keeps its space while the spinner is hidden, so the steps don't move. -->
    <div class="h-8">
      <AppSpinner v-if="restoring || creating" class="size-8" />
    </div>
    <div class="mt-base text-content-emphasis">
      <template v-if="restoring || creating" />
      <template v-else-if="restoreError || createError">
        <ErrorNote :message="(restoreError ?? createError?.message)!" />
        <WizardFooter back @back="back" />
      </template>
      <StepTarget v-else-if="step === 'target'" @next="next" />
      <StepVersion
        v-else-if="step === 'version'"
        :target="params.target!"
        @back="back"
        @next="next"
      />
      <StepPlatform
        v-else-if="step === 'platform'"
        :version="params.version!"
        @back="back"
        @next="next"
      />
      <StepBoard
        v-else-if="step === 'board'"
        :version="params.version!"
        @back="back"
        @next="next"
      />
      <StepArch
        v-else-if="step === 'arch'"
        :target="params.target!"
        :version="params.version!"
        :platform="params.platform"
        @back="back"
        @next="next"
      />
      <StepExtensions
        v-else-if="step === 'extensions'"
        :version="params.version!"
        @back="back"
        @next="next"
      />
      <StepCustomization
        v-else-if="step === 'customization'"
        :target="params.target!"
        :version="params.version!"
        :board="params.board"
        :secure-boot="params.secureBoot"
        @back="back"
        @next="next"
      />
      <StepFinal
        v-else-if="step === 'final' && params.schematicId"
        :schematic-id="params.schematicId"
        :target="params.target!"
        :version="params.version!"
        :arch="params.arch!"
        :platform="params.platform"
        :board="params.board"
        :secure-boot="params.secureBoot"
        @back="back"
      />
    </div>
    <LanguageSelect />
  </main>
</template>
