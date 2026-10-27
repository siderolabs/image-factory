// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { format, milliseconds } from 'date-fns'

/**
 * Renders a timestamp as YYYY-MM-DD HH:MM:SS +HH:MM in the reader's time zone. The format is fixed rather than
 * locale-dependent: the browser reports its UI language, not the system's regional settings, and a date that can
 * be read two ways is worse than one that reads the same everywhere. The offset is the one in force at that date,
 * so a date across a daylight-saving change carries its own.
 */
export function formatDateTime(date: Date): string {
  return format(date, 'yyyy-MM-dd HH:mm:ss xxx')
}

const durationUnits: Record<string, number> = {
  s: milliseconds({ seconds: 1 }),
  m: milliseconds({ minutes: 1 }),
  h: milliseconds({ hours: 1 }),
}

/**
 * Converts a Go duration such as 720h or 1h30m to milliseconds, or undefined for anything else. Only hours, minutes
 * and seconds are read, as no token lives for less. It's only used to preview what a duration works out to: the
 * server decides what is valid.
 */
export function parseGoDuration(text: string): number | undefined {
  const re = /(\d+(?:\.\d+)?)(s|m|h)/y
  let total = 0
  let at = 0

  while (at < text.length) {
    re.lastIndex = at

    const match = re.exec(text)
    if (!match) return undefined

    total += parseFloat(match[1]) * durationUnits[match[2]]
    at = re.lastIndex
  }

  return total > 0 ? total : undefined
}
