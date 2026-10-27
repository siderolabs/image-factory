// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import '@/index.css'

import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { createApp } from 'vue'

import App from '@/App.vue'
import { i18n, initLanguage } from '@/i18n'
import router from '@/router'

await initLanguage()

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Failures show right away; a failed query loads again the next time it's used.
      retry: false,
      refetchOnWindowFocus: false,
    },
  },
})

createApp(App).use(router).use(i18n).use(VueQueryPlugin, { queryClient }).mount('#app')
