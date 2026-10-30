// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { setupWorker } from 'msw/browser'

/**
 * Mocks the API in the unit tests, which run in a browser. vitest.setup.ts gives it the default handlers before each
 * test, so a test only overrides what it needs with `worker.use()`.
 */
export const worker = setupWorker()
