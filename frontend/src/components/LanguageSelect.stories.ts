// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import type { Meta, StoryObj } from '@storybook/vue3-vite'

import LanguageSelect from './LanguageSelect.vue'

/** Picking a language switches the whole UI, as the Language toolbar does. */
const meta: Meta<typeof LanguageSelect> = {
  component: LanguageSelect,
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
