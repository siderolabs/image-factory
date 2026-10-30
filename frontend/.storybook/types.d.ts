// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
import 'storybook/internal/csf'

import type { SetupWorker } from 'msw/browser'

declare module 'storybook/internal/csf' {
  interface StoryContext {
    msw: SetupWorker
  }
}
