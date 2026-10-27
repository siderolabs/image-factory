// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { describe, expect, test } from 'vitest'

import { languages } from '@/i18n'

const locales = import.meta.glob<Record<string, string>>('@/locales/*.json', {
  eager: true,
  import: 'default',
})

function messages(lang: string) {
  const found = Object.entries(locales).find(([path]) => path.endsWith(`/${lang}.json`))

  if (!found) throw new Error(`no messages for ${lang}`)

  return found[1]
}

/** The placeholders of each message, such as {link}, by message ID. */
function placeholders(messages: Record<string, string>) {
  return Object.fromEntries(
    Object.entries(messages).map(([key, message]) => [
      key,
      [...message.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort(),
    ]),
  )
}

const en = messages('en')

describe('locales', () => {
  test('every language has messages', () => {
    expect(Object.keys(locales)).toHaveLength(languages.length)
  })

  test.each(languages.filter((lang) => lang !== 'en'))('%s has the messages of en', (lang) => {
    const translated = messages(lang)

    expect(Object.keys(translated).sort()).toEqual(Object.keys(en).sort())
    expect(placeholders(translated)).toEqual(placeholders(en))
  })

  test.each(languages)('%s has no space before punctuation', (lang) => {
    const spaced = Object.entries(messages(lang)).filter(([, message]) => /\s[.,)]/.test(message))

    expect(spaced).toEqual([])
  })
})
