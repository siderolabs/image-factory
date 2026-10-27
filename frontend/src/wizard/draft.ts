// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { useSessionStorage } from '@vueuse/core'

import type { WizardParams } from '@/wizard/params'

/**
 * The choices last made on each step, which a step starts from when it's shown again. They're kept for the tab's
 * session, so a reload doesn't lose them. The URL holds the choices of the way through the wizard being shown, except
 * the embedded machine configuration, which may hold secrets and so is never put in the URL.
 */
export const draft = useSessionStorage<Partial<WizardParams> & { embeddedConfig?: string }>(
  'wizard-draft',
  {},
)
