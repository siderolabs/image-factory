// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
import '../src/index.css'

import { faker } from '@faker-js/faker'
import { apiHandlers } from '@mocks/msw/handlers'
import { type Preview } from '@storybook/vue3-vite'
import { startOfHour } from 'date-fns'
import { setupWorker } from 'msw/browser'
import { mswLoader } from 'msw-storybook-addon/csf3'
import { vueRouter } from 'storybook-vue3-router'
import { createMemoryHistory } from 'vue-router'
import { routes } from 'vue-router/auto-routes'

import { useTheme } from '../src/lib/theme'
import { applyTheme, parseThemeChoice } from '../src/lib/theme-core'

// Relative faker dates count back from the current hour rather than the current moment, so
// seeded stories render the same timestamps between runs. Set at load so mocks generated at
// import time get it too.
faker.setDefaultRefDate(() => startOfHour(new Date()))

const preview: Preview = {
  globalTypes: {
    theme: {
      description: 'Colour theme',
      toolbar: {
        title: 'Theme',
        icon: 'mirror',
        items: [
          { value: 'light', title: 'Light', icon: 'sun' },
          { value: 'dark', title: 'Dark', icon: 'moon' },
          { value: 'system', title: 'System', icon: 'browser' },
        ],
        dynamicTitle: true,
      },
    },
  },
  initialGlobals: {
    theme: 'system',
  },
  beforeEach({ msw }) {
    faker.seed(0)
    // Every story gets a mocked API, with fresh state. A story's own beforeEach runs after this one, so its
    // msw.use() overrides take precedence.
    msw.use(...apiHandlers())
  },
  loaders: [
    mswLoader(async () => {
      const worker = setupWorker()
      await worker.start({ onUnhandledFrame: 'bypass' })
      return worker
    }),
  ],
  decorators: [
    // Stories have no App.vue to apply the theme, so the toolbar sets the
    // choice that components like the theme menu read, and applies it.
    (_story, { globals }) => {
      const { theme, applied, setTheme } = useTheme()

      setTheme(parseThemeChoice(globals.theme))
      applyTheme(theme.value, applied.value)

      return { template: '<story />' }
    },
    vueRouter(undefined, {
      vueRouterOptions: {
        history: createMemoryHistory(),
        routes,
      },
    }),
  ],
  parameters: {
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },

    a11y: {
      // 'todo' - show a11y violations in the test UI only
      // 'error' - fail CI on a11y violations
      // 'off' - skip a11y checks entirely
      test: 'todo',
    },
  },
}

export default preview
