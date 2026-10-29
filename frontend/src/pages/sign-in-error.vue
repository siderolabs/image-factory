<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useRouteQuery } from '@vueuse/router'
import { computed } from 'vue'
import { Translation, useI18n } from 'vue-i18n'

const { t } = useI18n()

// Why the sign-in failed, as the backend reports it; anything else gets the generic message.
const reasons = ['cookie', 'rejected', 'account'] as const

const reason = useRouteQuery<string>('reason', '')

const message = computed(() => {
  const known = reasons.find((r) => r === reason.value)

  return t(`auth.login_error.reason.${known ?? 'unknown'}`)
})
</script>

<template>
  <main class="container mx-auto max-w-3xl px-base py-section">
    <h2 class="mb-compact type-page-title text-content-emphasis">
      {{ t('auth.login_error.title') }}
    </h2>
    <p class="mb-compact type-body text-content-default">{{ message }}</p>
    <!-- A real form post: the backend clears the session and hands over to the identity provider's sign-out. -->
    <form method="POST" action="/logout" class="type-body text-content-default">
      <Translation scope="global" keypath="auth.login_error.try_again">
        <template #signOut>
          <!-- prettier-ignore -->
          <button type="submit" class="cursor-pointer font-medium text-accent-text underline">{{ t('auth.login_error.signout') }}</button>
        </template>
      </Translation>
    </form>
  </main>
</template>
