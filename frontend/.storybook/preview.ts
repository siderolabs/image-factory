// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
import '../src/index.css'

import { faker } from '@faker-js/faker'
import { apiHandlers } from '@mocks/msw/handlers'
import { type Preview, setup } from '@storybook/vue3-vite'
import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { startOfHour } from 'date-fns'
import { setupWorker } from 'msw/browser'
import { mswLoader } from 'msw-storybook-addon/csf3'
import { vueRouter } from 'storybook-vue3-router'
import { createMemoryHistory } from 'vue-router'
import { routes } from 'vue-router/auto-routes'

import { config } from '../src/config'
import { i18n, type Language, languageNames, setLanguage } from '../src/i18n'
import { useTheme } from '../src/lib/theme'
import { applyTheme, parseThemeChoice } from '../src/lib/theme-core'
import { draft } from '../src/wizard/draft'

// Relative faker dates count back from the current hour rather than the current moment, so
// seeded stories render the same timestamps between runs. Set at load so mocks generated at
// import time get it too.
faker.setDefaultRefDate(() => startOfHour(new Date()))

// Storybook creates an app for each story, so each story starts with an empty query cache. Queries don't retry, as in
// main.ts, so that error stories show their error right away.
setup((app) => {
  app.use(i18n)
  app.use(VueQueryPlugin, {
    queryClient: new QueryClient({
      defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
    }),
  })
})

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
    lang: {
      description: 'Language',
      toolbar: {
        title: 'Language',
        icon: 'globe',
        items: Object.entries(languageNames).map(([value, title]) => ({ value, title })),
        dynamicTitle: true,
      },
    },
  },
  initialGlobals: {
    theme: 'system',
    lang: 'en',
  },
  beforeEach({ msw, parameters }) {
    faker.seed(0)

    // Every story gets a mocked API, with fresh state. A story's own beforeEach runs after this one, so its
    // msw.use() overrides take precedence.
    msw.use(...apiHandlers())

    // The wizard steps start from the choices made before, which a story sets in its own beforeEach.
    draft.value = {}

    // The server fills in the config on every request; a story sets what it needs with the config parameter.
    const defaults = { ...config }

    Object.assign(config, parameters.config)

    return () => {
      Object.assign(config, defaults)
    }
  },
  loaders: [
    mswLoader(async () => {
      const worker = setupWorker()
      await worker.start({ onUnhandledFrame: 'bypass' })
      return worker
    }),
    ({ globals }) => setLanguage(globals.lang as Language),
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
    // A story starts on the route of its route parameter.
    (story, context) =>
      vueRouter(undefined, {
        initialRoute: context.parameters.route,
        vueRouterOptions: {
          history: createMemoryHistory(),
          routes,
        },
      })(story, context),
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
