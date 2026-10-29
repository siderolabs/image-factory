<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import SignInLanding from '@/components/SignInLanding.vue'
import WizardPage from '@/components/wizard/WizardPage.vue'
import { config } from '@/config'

// With browser login, the backend lets signed-out visitors through to the root only, for the landing page.
const signedOut = config.logoutEnabled && !config.authenticated
</script>

<template>
  <SignInLanding v-if="signedOut" />
  <WizardPage v-else />
</template>
