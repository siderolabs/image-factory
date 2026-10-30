// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
import { fileURLToPath, URL } from 'node:url'

import type { StorybookConfig } from '@storybook/vue3-vite'

const config: StorybookConfig = {
  stories: ['../src/**/*.stories.ts'],
  addons: ['@storybook/addon-a11y', 'msw-storybook-addon'],
  framework: {
    name: '@storybook/vue3-vite',
    options: {},
  },
  staticDirs: ['./public'],
  async viteFinal(config) {
    const { mergeConfig } = await import('vite')

    return mergeConfig(config, {
      resolve: {
        alias: {
          '@mocks': fileURLToPath(new URL('../mocks', import.meta.url)),
        },
      },
    })
  },
}

export default config
