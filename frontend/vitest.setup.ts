// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { apiHandlers } from '@mocks/msw/handlers'
import { worker } from '@mocks/msw/worker'
import { afterAll, beforeAll, beforeEach } from 'vitest'

// The test runner loads its own modules through the page too, so requests without a handler go to the network.
beforeAll(() => worker.start({ quiet: true, onUnhandledFrame: 'bypass' }))

// Every test starts with fresh handlers, which drops the overrides of the previous test, and its schematics and
// tokens.
beforeEach(() => worker.resetHandlers(...apiHandlers()))

afterAll(() => worker.stop())
