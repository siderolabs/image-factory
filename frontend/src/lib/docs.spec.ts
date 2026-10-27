// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { describe, expect, test } from 'vitest'

import { docsURL } from '@/lib/docs'

describe('docs', () => {
  test('links a page of the documentation for a version', () => {
    expect(docsURL('1.13.2', '/getting-started/support-matrix/')).toBe(
      'https://docs.siderolabs.com/talos/v1.13/getting-started/support-matrix/',
    )
  })
})
