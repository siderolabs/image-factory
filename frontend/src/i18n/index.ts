// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { createI18n } from 'vue-i18n'

/** The UI languages, in the order the language picker lists them. */
export const languages = ['en', 'fr', 'pl', 'ru'] as const

export type Language = (typeof languages)[number]

/** Each language's name for itself, which is how the language picker lists it. */
export const languageNames: Record<Language, string> = {
  en: 'English',
  fr: 'Français',
  pl: 'Polski',
  ru: 'Русский',
}

type Messages = Record<string, string>

// Each language loads when it is first used.
const loaders: Record<Language, () => Promise<{ default: Messages }>> = {
  en: () => import('@/locales/en.json'),
  fr: () => import('@/locales/fr.json'),
  pl: () => import('@/locales/pl.json'),
  ru: () => import('@/locales/ru.json'),
}

function isLanguage(lang: string): lang is Language {
  return (languages as readonly string[]).includes(lang)
}

/**
 * The backend picks the language from ?lang=, the lang cookie and Accept-Language, and puts it on <html lang>.
 */
function initialLanguage(): Language {
  const lang = document.documentElement.lang

  return isLanguage(lang) ? lang : 'en'
}

export const i18n = createI18n({
  legacy: false,
  locale: 'en',
  // Every language has every message, which src/i18n/index.spec.ts checks; a missing one shows its key.
  fallbackLocale: false,
  messages: {} as Partial<Record<Language, Messages>>,
  // Message IDs such as "page.header" sit next to "page.header.enterprise", so they are looked up as flat keys
  // rather than as nested paths.
  messageResolver: (messages, key) => (messages as Record<string, string>)[key] ?? null,
})

async function loadLanguage(lang: Language) {
  if (i18n.global.availableLocales.includes(lang)) return

  const { default: messages } = await loaders[lang]()

  i18n.global.setLocaleMessage(lang, messages)
}

/** Loads the language the backend picked for the page and switches to it, before the app renders. */
export async function initLanguage() {
  const lang = initialLanguage()

  await loadLanguage(lang)
  i18n.global.locale.value = lang
}

/**
 * Switches the UI language, and remembers it in the lang cookie, which the backend reads.
 */
export async function setLanguage(lang: Language) {
  await loadLanguage(lang)
  i18n.global.locale.value = lang
  document.documentElement.lang = lang

  // The backend sets the cookie when it serves the page with ?lang=.
  await fetch(`/?lang=${encodeURIComponent(lang)}`, { method: 'HEAD', credentials: 'same-origin' })
}
