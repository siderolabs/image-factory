// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { type Config } from 'prettier'
import { type PluginOptions } from 'prettier-plugin-tailwindcss'

const config: Config & PluginOptions = {
  semi: false,
  singleQuote: true,
  printWidth: 100,
  plugins: ['prettier-plugin-tailwindcss'],
  htmlWhitespaceSensitivity: 'ignore',
  tailwindStylesheet: './src/index.css',
}

export default config
