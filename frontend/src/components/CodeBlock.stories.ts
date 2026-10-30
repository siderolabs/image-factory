// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { vanillaSchematic } from '@mocks/fixtures'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import CodeBlock from './CodeBlock.vue'

const meta: Meta<typeof CodeBlock> = {
  component: CodeBlock,
  args: {
    code: `factory.talos.dev/metal-installer/${vanillaSchematic.id}:v1.13.4`,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const Block: Story = {
  args: {
    block: true,
    code: `apiVersion: v1alpha1
kind: RegistryAuthConfig
name: "factory.talos.dev"
username: "0f8a3c2e-6b1d-4e7a-9c5f-2d8b4a6e1f3c"
password: "eyJhbGciOiJFUzI1NiJ9..."
`,
  },
}
