<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useMutation } from '@tanstack/vue-query'
import { addMilliseconds } from 'date-fns'
import { computed, onMounted, ref, useId, useTemplateRef } from 'vue'
import { useI18n } from 'vue-i18n'

import { APIError, type Schemas } from '@/api/client'
import { createTokenMutation } from '@/api/queries'
import AppInput from '@/components/AppInput.vue'
import AppSpinner from '@/components/AppSpinner.vue'
import ErrorNote from '@/components/ErrorNote.vue'
import ModalDialog from '@/components/ModalDialog.vue'
import { formatDateTime, parseGoDuration } from '@/lib/time'

const { actors } = defineProps<{
  actors: Schemas['APITokenActor'][]
}>()

const emit = defineEmits<{
  close: []
  created: [token: Schemas['APITokenCreated']]
}>()

const { t } = useI18n()
const id = useId()
const nameInput = useTemplateRef('name-input')

const name = ref('')
const ttl = ref('')
// Talos is the narrowest actor, so it's the default.
const actor = ref('talos')
const {
  mutate: createToken,
  isPending: pending,
  error: createError,
} = useMutation(createTokenMutation())

// 409 means the identity has as many tokens as it may.
const error = computed(() => {
  const err = createError.value

  if (!err) return undefined

  return err instanceof APIError && err.message === 'HTTP 409'
    ? t('tokens.error.limit')
    : err.message
})

const selectedActor = computed(() => actors.find((a) => a.id === actor.value))

// The date that the lifetime works out to, so nobody has to count hours to find out what 2160h means. An empty
// field takes the server's default, whose length the page doesn't know.
const expires = computed(() => {
  const ms = parseGoDuration(ttl.value.trim())

  return ms === undefined ? undefined : formatDateTime(addMilliseconds(new Date(), ms))
})

onMounted(() => nameInput.value?.focus())

function create() {
  if (pending.value || !name.value.trim()) return

  createToken(
    {
      name: name.value.trim(),
      stored: true,
      actor: actor.value as Schemas['APITokenCreateRequest']['actor'],
      ...(ttl.value.trim() ? { ttl: ttl.value.trim() } : {}),
    } as Schemas['APITokenCreateRequest'],
    { onSuccess: (created) => emit('created', created) },
  )
}
</script>

<template>
  <!-- A create in flight keeps the dialog open: closing it would let the next click mint a second token. -->
  <ModalDialog :title="t('tokens.create.button')" :locked="pending" @close="emit('close')">
    <ErrorNote v-if="error" :message="error" />
    <form @submit.prevent="create">
      <label :for="`${id}-name`" class="mb-micro block type-label text-content-secondary">
        {{ t('tokens.create.name_label') }}
      </label>
      <AppInput
        :id="`${id}-name`"
        ref="name-input"
        v-model="name"
        type="text"
        required
        class="mb-compact"
      />

      <label :for="`${id}-ttl`" class="mb-micro block type-label text-content-secondary">
        {{ t('tokens.create.ttl_label') }}
      </label>
      <AppInput :id="`${id}-ttl`" v-model="ttl" type="text" autocomplete="off" placeholder="720h" />
      <div class="mt-micro mb-compact type-meta text-content-secondary">
        <p>{{ t('tokens.create.ttl_help') }}</p>
        <p class="mt-micro">{{ t('tokens.create.ttl_examples') }}</p>
        <p v-if="expires" class="mt-micro">
          {{ t('tokens.create.ttl_expires', { time: expires }) }}
        </p>
      </div>

      <fieldset class="mb-compact">
        <legend class="mb-tight block type-label text-content-secondary">
          {{ t('tokens.create.actor_label') }}
        </legend>
        <div class="grid grid-cols-1 gap-tight sm:grid-cols-2">
          <label
            v-for="a in actors"
            :key="a.id"
            class="relative block cursor-pointer rounded-lg border border-border-default p-snug type-label hover:border-border-accent"
          >
            <input v-model="actor" type="radio" name="actor" :value="a.id" class="peer sr-only" />
            <span class="block type-control text-content-emphasis">
              {{ t(`tokens.create.actor_${a.id}`) }}
            </span>
            <span class="mt-micro block type-meta text-content-secondary">
              {{ t(`tokens.create.actor_${a.id}_help`) }}
            </span>
            <span
              class="pointer-events-none absolute inset-0 rounded-lg border border-transparent peer-checked:border-border-accent peer-focus-visible:shadow-focus"
            ></span>
          </label>
        </div>

        <!--
          The scopes of the selected actor, below the grid rather than in each card: the lists are long enough that
          putting them in the cards either widens the dialog or makes the grid jump as the selection moves.
        -->
        <div v-if="selectedActor" class="mt-tight type-meta text-content-secondary">
          <p>
            <span class="me-micro type-label">{{ t('tokens.create.actor_scopes') }}</span>
            <span class="type-mono wrap-break-word">{{ selectedActor.scopes.join(' ') }}</span>
          </p>
          <p v-if="selectedActor.issuable_scopes.length" class="mt-micro">
            <span class="me-micro type-label">{{ t('tokens.create.actor_issuable') }}</span>
            <span class="type-mono wrap-break-word">
              {{ selectedActor.issuable_scopes.join(' ') }}
            </span>
          </p>
        </div>
      </fieldset>

      <div class="flex justify-end gap-tight">
        <button
          type="button"
          :disabled="pending"
          class="rounded-lg border border-border-default bg-surface-card px-compact py-tight type-control text-content-emphasis hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-50"
          @click="emit('close')"
        >
          {{ t('tokens.create.cancel') }}
        </button>
        <button
          type="submit"
          :disabled="pending"
          class="inline-flex items-center gap-tight rounded-lg bg-accent-fill px-compact py-tight text-center type-control text-content-on-accent hover:bg-accent-fill-hover active:bg-accent-fill-active disabled:cursor-not-allowed disabled:opacity-50"
        >
          <AppSpinner v-if="pending" class="h-3.5 w-3.5" />
          {{ t('tokens.create.button') }}
        </button>
      </div>
    </form>
  </ModalDialog>
</template>
