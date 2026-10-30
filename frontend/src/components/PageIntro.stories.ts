// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import type { Meta, StoryObj } from '@storybook/vue3-vite'

import PageIntro from './PageIntro.vue'

const meta: Meta<typeof PageIntro> = {
  component: PageIntro,
  parameters: {
    config: { version: 'v1.0.0' },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
