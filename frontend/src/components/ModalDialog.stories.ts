// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import ModalDialog from './ModalDialog.vue'

const meta: Meta<typeof ModalDialog> = {
  component: ModalDialog,
  args: {
    title: 'Revoke Token',
    onClose: fn(),
  },
  parameters: {
    layout: 'fullscreen',
  },
  render: (args) => ({
    components: { ModalDialog },
    setup: () => ({ args }),
    template: `
      <ModalDialog v-bind="args">
        <p class="type-body text-content-default">
          Anything using this token loses access right away. This can't be undone.
        </p>
      </ModalDialog>
    `,
  }),
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const Wide: Story = {
  args: {
    width: 'lg',
  },
}

/** Stays open on Escape and on clicking outside, as while its request is in flight. */
export const Locked: Story = {
  args: {
    locked: true,
  },
}
