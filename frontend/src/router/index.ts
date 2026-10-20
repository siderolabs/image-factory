// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { createRouter, createWebHistory } from 'vue-router'
import { handleHotUpdate, routes } from 'vue-router/auto-routes'

const router = createRouter({
  history: createWebHistory(),
  routes,
})

if (import.meta.hot) {
  handleHotUpdate(router)
}

export default router
