// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { describe, expect, test } from 'vitest'

import { groupVersions, shortVersion } from '@/lib/versions'

describe('groupVersions', () => {
  test('groups by minor release, newest first, pre-releases after their release', () => {
    expect(
      groupVersions([
        'v1.12.0',
        'v1.12.1',
        'v1.13.0-alpha.1',
        'v1.13.0-beta.0',
        'v1.13.0',
        'v1.14.0-alpha.0',
      ]),
    ).toEqual({
      groups: [
        { label: '1.14-pre', versions: ['1.14.0-alpha.0'] },
        { label: '1.13', versions: ['1.13.0'] },
        { label: '1.13-pre', versions: ['1.13.0-beta.0', '1.13.0-alpha.1'] },
        { label: '1.12', versions: ['1.12.1', '1.12.0'] },
      ],
      latestStable: '1.13.0',
    })
  })

  test('has no latest stable version without a release', () => {
    expect(groupVersions(['v1.14.0-alpha.0']).latestStable).toBeUndefined()
  })
})

describe('shortVersion', () => {
  test('shortens to the documentation version', () => {
    expect(shortVersion('1.13.2')).toBe('v1.13')
    expect(shortVersion('v1.14.0-alpha.1')).toBe('v1.14')
  })
})
