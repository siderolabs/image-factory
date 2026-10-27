<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { onKeyStroke, useScrollLock } from '@vueuse/core'
import { computed, ref, watchEffect } from 'vue'
import { Translation, useI18n } from 'vue-i18n'

import { scanReportQuery } from '@/api/queries'
import AppIcon from '@/components/AppIcon.vue'
import { config } from '@/config'
import { enterpriseDocsURL } from '@/lib/docs'
import {
  bestURL,
  fixVersions,
  type Match,
  preferredCVE,
  severities,
  type Severity,
  vexStatements,
} from '@/wizard/scan'

const { scan } = defineProps<{
  /** Base URL of the vulnerability scan reports; unset outside Enterprise. */
  scan?: string
}>()

const { t } = useI18n()

const reports = computed(() => {
  const base = scan?.replace(/\/$/, '')

  return base
    ? {
        json: `${base}/report.json`,
        sarif: `${base}/report.sarif`,
        cdx: `${base}/report.cdx`,
        table: `${base}/report.table`,
      }
    : undefined
})

const {
  data: report,
  isLoading,
  error,
} = useQuery(() => ({
  ...scanReportQuery(reports.value?.json ?? ''),
  enabled: !!reports.value,
}))

const bySeverity = computed(() => {
  const groups = new Map<Severity, Match[]>()

  for (const match of report.value?.matches ?? []) {
    const severity = (match.vulnerability?.severity ?? 'Unknown') as Severity

    groups.set(severity, [...(groups.get(severity) ?? []), match])
  }

  return groups
})

const total = computed(() => report.value?.matches?.length ?? 0)

const dots: Record<Severity, string> = {
  Critical: 'bg-status-danger-default',
  High: 'bg-status-warning-default',
  Medium: 'bg-status-warning-default',
  Low: 'bg-status-info-default',
  Negligible: 'bg-content-muted',
  Unknown: 'bg-content-muted',
}

const shown = ref<Severity>()

const rows = computed(() =>
  [...(shown.value ? (bySeverity.value.get(shown.value) ?? []) : [])].sort((a, b) =>
    (a.artifact?.name ?? '').localeCompare(b.artifact?.name ?? ''),
  ),
)

onKeyStroke('Escape', () => (shown.value = undefined))

// The page behind the dialog doesn't scroll while it's open.
const scrollLocked = useScrollLock(document.body)

watchEffect(() => (scrollLocked.value = !!shown.value))
</script>

<template>
  <section
    class="mb-base rounded-md border border-border-default bg-surface-inset p-compact type-body"
  >
    <h2 class="mb-tight type-section-title">{{ t('final.scan.title') }}</h2>
    <template v-if="config.enterprise && reports">
      <p v-if="isLoading" class="mb-tight text-content-default">
        {{ t('final.scan.loading') }}
      </p>
      <p
        v-else-if="error"
        class="status-glyph status-failed mb-tight type-body text-content-default"
      >
        {{ t('final.scan.error', { error: error.message }) }}
      </p>
      <div v-else class="mb-tight">
        <p class="mb-tight">
          {{ total }} vulnerabilit{{ total === 1 ? 'y' : 'ies' }} after VEX suppression
        </p>
        <ul class="mb-tight flex flex-wrap gap-snug">
          <template v-for="severity in severities" :key="severity">
            <li v-if="bySeverity.get(severity)?.length">
              <button
                type="button"
                class="inline-flex cursor-pointer items-center gap-micro rounded border border-border-default bg-surface-card px-tight py-micro type-control text-content-default hover:bg-surface-hover"
                :title="`Click to view ${severity} vulnerabilities`"
                @click="shown = severity"
              >
                <span class="size-2 rounded-full" :class="dots[severity]"></span>
                <AppIcon name="search" class="mr-micro inline-block h-3 w-3" />
                {{ severity }}: {{ bySeverity.get(severity)?.length }}
              </button>
            </li>
          </template>
        </ul>
      </div>
      <p class="type-meta">
        {{ t('final.scan.download') }}
        <a class="ml-micro underline" :href="reports.json">JSON</a>
        <a class="ml-micro underline" :href="reports.sarif">SARIF</a>
        <a class="ml-micro underline" :href="reports.cdx">CycloneDX</a>
        <a class="ml-micro underline" :href="reports.table">Table</a>
      </p>
    </template>
    <p v-else>
      <Translation scope="global" keypath="final.scan.enterprise_text">
        <template #link>
          <a :href="enterpriseDocsURL" target="_blank" rel="noopener noreferrer" class="underline">
            {{ t('final.enterprise.learn_more') }}
          </a>
        </template>
      </Translation>
    </p>
  </section>

  <div
    v-if="shown"
    class="fixed inset-0 z-50 flex items-center justify-center bg-surface-scrim p-compact"
    @click.self="shown = undefined"
  >
    <div
      role="dialog"
      aria-modal="true"
      class="relative flex h-[85vh] w-full max-w-4xl flex-col overflow-hidden rounded-md border border-border-default bg-surface-card shadow-modal"
    >
      <div
        class="flex flex-none items-center justify-between border-b border-border-default px-compact py-tight"
      >
        <h3 class="type-section-title">{{ shown }} ({{ rows.length }})</h3>
        <button
          type="button"
          class="cursor-pointer border-0 bg-transparent type-control text-content-secondary hover:text-content-emphasis"
          aria-label="Close"
          @click="shown = undefined"
        >
          &times;
        </button>
      </div>
      <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain p-compact">
        <table class="w-full table-fixed border-collapse type-body">
          <thead>
            <tr class="border-b border-border-default text-left">
              <th class="w-40 py-micro pr-snug type-overline text-content-secondary">CVE</th>
              <th class="w-[18%] py-micro pr-snug type-overline text-content-secondary">Package</th>
              <th class="w-28 py-micro pr-snug type-overline text-content-secondary">Version</th>
              <th class="w-[16%] py-micro pr-snug type-overline text-content-secondary">Fix</th>
              <th class="py-micro type-overline text-content-secondary">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(match, index) in rows"
              :key="index"
              class="border-b border-border-subtle align-top"
            >
              <td class="py-micro pr-snug type-mono whitespace-nowrap underline">
                <a
                  v-if="bestURL(match)"
                  :href="bestURL(match)"
                  target="_blank"
                  rel="noopener noreferrer"
                  :title="bestURL(match)"
                >
                  {{ preferredCVE(match) }}
                  <AppIcon name="external" class="ml-micro inline-block h-3 w-3 align-text-top" />
                </a>
                <template v-else>{{ preferredCVE(match) }}</template>
              </td>
              <td class="py-micro pr-snug type-mono break-all">{{ match.artifact?.name }}</td>
              <td class="py-micro pr-snug type-mono whitespace-nowrap">
                {{ match.artifact?.version }}
              </td>
              <td class="py-micro pr-snug type-mono break-all">{{ fixVersions(match) }}</td>
              <td class="py-micro wrap-break-word">
                <div
                  v-if="vexStatements(match).length"
                  class="mb-tight border-b border-border-subtle pb-tight"
                >
                  <div class="type-overline text-content-secondary">VEX</div>
                  <p v-for="note in vexStatements(match)" :key="note" class="whitespace-pre-wrap">
                    {{ note }}
                  </p>
                </div>
                <p class="whitespace-pre-wrap">{{ match.vulnerability?.description }}</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
