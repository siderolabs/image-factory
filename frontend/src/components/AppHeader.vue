<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import ThemeMenu from '@/components/ThemeMenu.vue'
import { config } from '@/config'

const { t } = useI18n()
const route = useRoute()

const navLink =
  'rounded-md px-snug py-tight text-content-default transition-colors hover:bg-surface-hover hover:text-content-emphasis'
</script>

<template>
  <header class="sticky top-0 z-10 border-b border-border-default bg-surface-chrome">
    <div
      class="container mx-auto flex max-w-3xl items-center justify-between gap-compact px-base py-snug"
    >
      <h1 class="type-page-title text-content-emphasis">
        {{ t(config.enterprise ? 'page.header.enterprise' : 'page.header') }}
      </h1>
      <nav class="flex items-center gap-micro type-control">
        <template v-if="config.authenticated">
          <RouterLink v-if="route.path === '/ui/tokens'" to="/" :class="navLink">
            {{ t('tokens.back_home') }}
          </RouterLink>
          <RouterLink v-else-if="config.tokensEnabled" to="/ui/tokens" :class="navLink">
            {{ t('tokens.nav_link') }}
          </RouterLink>
          <div
            v-if="config.logoutEnabled && (config.tokensEnabled || route.path === '/ui/tokens')"
            class="h-4 w-px bg-border-default"
          ></div>
          <form v-if="config.logoutEnabled" method="POST" action="/logout">
            <button type="submit" :class="navLink">{{ t('nav.logout') }}</button>
          </form>
        </template>
        <ThemeMenu />
      </nav>
    </div>
  </header>
</template>
