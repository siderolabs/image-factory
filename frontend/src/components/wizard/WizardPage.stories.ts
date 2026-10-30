// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { vanillaSchematic } from '@mocks/fixtures'
import { apiError } from '@mocks/msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import WizardPage from './WizardPage.vue'

/** The wizard keeps its choices in the URL, so each story starts on the route of a step. */
const meta: Meta<typeof WizardPage> = {
  component: WizardPage,
  parameters: {
    layout: 'fullscreen',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Start: Story = {}

export const Version: Story = {
  parameters: {
    route: '/?target=metal',
  },
}

export const Platform: Story = {
  parameters: {
    route: '/?target=cloud&version=1.13.4',
  },
}

export const Board: Story = {
  parameters: {
    route: '/?target=sbc&version=1.13.4',
  },
}

export const Architecture: Story = {
  parameters: {
    route: '/?target=metal&version=1.13.4&platform=metal',
  },
}

export const Extensions: Story = {
  parameters: {
    route: '/?target=metal&version=1.13.4&arch=amd64&platform=metal',
  },
}

export const Customization: Story = {
  parameters: {
    route: '/?target=metal&version=1.13.4&arch=amd64&platform=metal&extensions=siderolabs/gvisor',
  },
}

/** With every choice made, the wizard creates the schematic, then links to it. */
export const Final: Story = {
  parameters: {
    route:
      '/?target=metal&version=1.13.4&arch=amd64&platform=metal&extensions=siderolabs/gvisor&cmdline-set=true',
  },
}

/** A schematic link restores the choices that the schematic records. */
export const SchematicLink: Story = {
  parameters: {
    route: `/?target=metal&version=1.13.4&arch=amd64&platform=metal&schematic-id=${vanillaSchematic.id}`,
  },
}

export const CreateError: Story = {
  parameters: Final.parameters,
  beforeEach({ msw }) {
    msw.use(apiError('post', '/schematics', 400, 'invalid extension "siderolabs/gvisor"'))
  },
}

export const UnknownSchematic: Story = {
  parameters: {
    route: '/?target=metal&version=1.13.4&arch=amd64&platform=metal&schematic-id=unknown',
  },
}
