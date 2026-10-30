// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import CheckboxCard from './CheckboxCard.vue'

const meta: Meta<typeof CheckboxCard> = {
  component: CheckboxCard,
  args: {
    'onUpdate:modelValue': fn(),
  },
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  args: {
    value: faker.commerce.isbn(),
    label: faker.commerce.productName(),
    description: faker.commerce.productDescription(),
  },
}
