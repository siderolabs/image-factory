// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
import 'storybook/internal/csf'

import type { SetupWorker } from 'msw/browser'

import type { Config } from '../src/config'

declare module 'storybook/internal/csf' {
  interface StoryContext {
    msw: SetupWorker
  }

  interface Parameters {
    /** Server configuration for the story, over the defaults that apply without a backend. */
    config?: Partial<Config>
    /** The route the story starts on, such as /ui/tokens or /?target=metal. */
    route?: string
  }
}
