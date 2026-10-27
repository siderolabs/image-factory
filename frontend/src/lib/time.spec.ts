// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { describe, expect, test } from 'vitest'

import { formatDateTime, parseGoDuration } from '@/lib/time'

describe('parseGoDuration', () => {
  test('reads Go durations', () => {
    expect(parseGoDuration('720h')).toBe(720 * 3_600_000)
    expect(parseGoDuration('1h30m')).toBe(90 * 60_000)
    expect(parseGoDuration('1.5h')).toBe(90 * 60_000)
  })

  test('rejects anything else', () => {
    expect(parseGoDuration('')).toBeUndefined()
    expect(parseGoDuration('30d')).toBeUndefined()
    expect(parseGoDuration('500ms')).toBeUndefined()
    expect(parseGoDuration('h')).toBeUndefined()
    expect(parseGoDuration('0s')).toBeUndefined()
  })
})

describe('formatDateTime', () => {
  test('writes the date, time and UTC offset', () => {
    expect(formatDateTime(new Date(2026, 0, 2, 3, 4, 5))).toMatch(
      /^2026-01-02 03:04:05 [+-]\d{2}:\d{2}$/,
    )
  })
})
