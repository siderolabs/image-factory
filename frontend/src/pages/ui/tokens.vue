<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useMutation, useMutationState, useQuery } from '@tanstack/vue-query'
import { formatTimeAgoIntl, type TimeAgoUnit, useIntervalFn, useNow, useTitle } from '@vueuse/core'
import { milliseconds } from 'date-fns'
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { Schemas } from '@/api/client'
import { revokeTokenMutation, tokenActorsQuery, tokensQuery } from '@/api/queries'
import AppIcon from '@/components/AppIcon.vue'
import AppSpinner from '@/components/AppSpinner.vue'
import CodeBlock from '@/components/CodeBlock.vue'
import ErrorNote from '@/components/ErrorNote.vue'
import ModalDialog from '@/components/ModalDialog.vue'
import TokenCreateDialog from '@/components/tokens/TokenCreateDialog.vue'
import { formatDateTime } from '@/lib/time'

const { t, locale } = useI18n()

// The backend titles index.html for the wizard; this page has its own title while it's open.
useTitle(() => t('tokens.page_title', { app: t('app.name') }))

const { data: actors, error: actorsError } = useQuery(tokenActorsQuery())
const { data: list, isLoading: loading, error: listError } = useQuery(tokensQuery())
const { mutateAsync: revokeToken } = useMutation(revokeTokenMutation())

const tokens = computed(() => list.value?.tokens ?? (list.value ? [] : undefined))
const revokeError = ref<string>()

const error = computed(
  () =>
    revokeError.value ??
    (listError.value ? t('tokens.error.load', { error: listError.value.message }) : undefined),
)

// Tokens revoked from this page. The revoke lands before the listing behind it catches up, and without this a
// reload would put back a row that is already gone.
const revoked = reactive(new Set<string>())
// The tokens whose revoke is still running.
const revoking = useMutationState({
  filters: { mutationKey: revokeTokenMutation().mutationKey, status: 'pending' },
  select: (mutation) => mutation.state.variables as string,
})

const rows = computed(() => (tokens.value ?? []).filter((tok) => !revoked.has(tok.id)))

const creating = ref(false)
const created = ref<Schemas['APITokenCreated']>()

function onCreated(token: Schemas['APITokenCreated']) {
  creating.value = false
  created.value = token
}

// The RegistryAuthConfig document that a Talos node needs to authenticate its registry pulls with a token that
// can read images, for this factory's own host, which serves both the page and the registry.
const patch = computed(() => {
  const token = created.value

  if (!token?.scopes.includes('image:read')) return undefined

  return [
    'apiVersion: v1alpha1',
    'kind: RegistryAuthConfig',
    `name: ${JSON.stringify(window.location.host)}`,
    `username: ${JSON.stringify(token.org_id)}`,
    `password: ${JSON.stringify(token.token)}`,
    '',
  ].join('\n')
})

const revokeTarget = ref<string>()

async function revoke() {
  const id = revokeTarget.value

  revokeTarget.value = undefined

  if (!id) return

  revokeError.value = undefined

  try {
    await revokeToken(id)

    revoked.add(id)
  } catch (err) {
    revokeError.value = t('tokens.error.revoke', {
      error: err instanceof Error ? err.message : String(err),
    })
  }
}

function permissions(token: Schemas['APIToken']) {
  const issuable = token.issuable_scopes ?? []

  return token.scopes.join(' ') + (issuable.length ? ` → ${issuable.join(' ')}` : '')
}

// Ticks so that the time left keeps counting down while the page is open.
const now = useNow({ scheduler: (cb) => useIntervalFn(cb, milliseconds({ seconds: 30 })) })

// Months and weeks are left out, so that a 720h token reads "in 30 days" rather than "in 1 month". A year is
// 365 days, as token lifetimes count it (8760h); date-fns' { years: 1 } is 365.2425.
const expiryUnits: TimeAgoUnit[] = [
  { name: 'year', ms: milliseconds({ days: 365 }) },
  { name: 'day', ms: milliseconds({ days: 1 }) },
  { name: 'hour', ms: milliseconds({ hours: 1 }) },
  { name: 'minute', ms: milliseconds({ minutes: 1 }) },
  { name: 'second', ms: milliseconds({ seconds: 1 }) },
]

function expiry(token: Schemas['APIToken']) {
  const expires = new Date(token.expires_at)

  if (expires <= now.value) return t('tokens.table.expired')

  return formatTimeAgoIntl(
    expires,
    { locale: locale.value, units: expiryUnits, relativeTimeFormatOptions: { numeric: 'always' } },
    now.value,
  )
}
</script>

<template>
  <main class="container mx-auto max-w-3xl px-base py-section">
    <div class="mb-compact flex items-center justify-between gap-compact">
      <h2 class="type-page-title text-content-emphasis">{{ t('tokens.title') }}</h2>
      <button
        type="button"
        :disabled="!actors"
        class="inline-flex shrink-0 items-center gap-tight rounded-lg bg-accent-fill px-compact py-tight text-center type-control text-content-on-accent transition-colors hover:bg-accent-fill-hover active:bg-accent-fill-active disabled:cursor-not-allowed disabled:opacity-50"
        @click="creating = true"
      >
        <AppIcon name="plus" class="h-3.5 w-3.5" />
        {{ t('tokens.create.button') }}
      </button>
    </div>
    <p class="mb-base type-body text-content-default">{{ t('tokens.description') }}</p>

    <ErrorNote v-if="error" :message="error" />
    <ErrorNote v-if="actorsError" :message="actorsError.message" />

    <!-- The listing can take seconds; an empty table in the meantime would read as "no tokens". -->
    <div
      v-if="loading && !tokens"
      class="mb-base flex items-center gap-tight type-body text-content-secondary"
    >
      <AppSpinner class="h-4 w-4" />
      {{ t('tokens.loading') }}
    </div>
    <template v-else-if="tokens">
      <table v-if="rows.length" class="mb-base w-full type-body">
        <thead>
          <tr class="border-b border-border-default text-left">
            <th class="py-tight pr-snug type-overline text-content-secondary">
              {{ t('tokens.table.name') }}
            </th>
            <th class="py-tight pr-snug type-overline text-content-secondary">
              {{ t('tokens.table.permissions') }}
            </th>
            <th class="py-tight pr-snug type-overline text-content-secondary">
              {{ t('tokens.table.created') }}
            </th>
            <th class="py-tight pr-snug type-overline text-content-secondary">
              {{ t('tokens.table.expires') }}
            </th>
            <th class="py-tight type-overline text-content-secondary"></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="token in rows"
            :key="token.id"
            class="border-b border-border-subtle"
            :class="{ 'opacity-50': revoking.includes(token.id) }"
          >
            <td class="py-tight pr-snug type-body-strong text-content-emphasis">
              {{ token.name }}
            </td>
            <td class="py-tight pr-snug type-mono">{{ permissions(token) }}</td>
            <td class="py-tight pr-snug whitespace-nowrap">
              {{ formatDateTime(new Date(token.created_at)) }}
            </td>
            <td class="py-tight pr-snug" :title="formatDateTime(new Date(token.expires_at))">
              {{ expiry(token) }}
            </td>
            <td class="py-tight text-right">
              <button
                type="button"
                :disabled="revoking.includes(token.id)"
                class="type-control text-status-danger-text hover:underline disabled:cursor-not-allowed disabled:no-underline disabled:opacity-50"
                @click="revokeTarget = token.id"
              >
                {{
                  t(revoking.includes(token.id) ? 'tokens.revoke.pending' : 'tokens.revoke.button')
                }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-else class="mb-base type-body text-content-secondary">{{ t('tokens.empty') }}</p>
    </template>

    <TokenCreateDialog
      v-if="creating && actors"
      :actors="actors"
      @close="creating = false"
      @created="onCreated"
    />

    <ModalDialog
      v-if="created"
      :title="t('tokens.secret.title')"
      width="lg"
      @close="created = undefined"
    >
      <p class="status-glyph status-warning mb-compact type-body text-content-default">
        {{ t('tokens.secret.warning') }}
      </p>
      <p class="mb-micro type-label text-content-secondary">{{ t('tokens.secret.identity') }}</p>
      <CodeBlock class="mb-compact" :code="created.org_id" block />
      <p class="mb-micro type-label text-content-secondary">{{ t('tokens.secret.token') }}</p>
      <CodeBlock class="mb-compact" :code="created.token" block />
      <template v-if="patch">
        <p class="mb-micro type-label text-content-secondary">{{ t('tokens.secret.patch') }}</p>
        <CodeBlock class="mb-compact" :code="patch" block />
      </template>
      <div class="flex justify-end">
        <button
          type="button"
          class="rounded-lg border border-border-default bg-surface-card px-compact py-tight type-control text-content-emphasis hover:bg-surface-hover"
          @click="created = undefined"
        >
          {{ t('tokens.secret.close') }}
        </button>
      </div>
    </ModalDialog>

    <ModalDialog
      v-if="revokeTarget"
      :title="t('tokens.revoke.title')"
      @close="revokeTarget = undefined"
    >
      <p class="mb-compact type-body text-content-default">{{ t('tokens.revoke.confirm') }}</p>
      <div class="flex justify-end gap-tight">
        <button
          type="button"
          class="rounded-lg border border-border-default bg-surface-card px-compact py-tight type-control text-content-emphasis hover:bg-surface-hover"
          @click="revokeTarget = undefined"
        >
          {{ t('tokens.create.cancel') }}
        </button>
        <button
          type="button"
          class="rounded-lg bg-status-danger-fill px-compact py-tight text-center type-control text-status-danger-on-fill"
          @click="revoke"
        >
          {{ t('tokens.revoke.button') }}
        </button>
      </div>
    </ModalDialog>
  </main>
</template>
