<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { Schemas } from '@/api/client'
import CodeBlock from '@/components/CodeBlock.vue'
import { config } from '@/config'
import { docsURL } from '@/lib/docs'
import type { Target } from '@/wizard/params'

const { asset, target, version } = defineProps<{
  asset: Schemas['Asset']
  target: Target
  version: string
}>()

const { t } = useI18n()

const label = computed(() => {
  switch (asset.kind) {
    case 'iso':
      return asset.secureBoot ? 'SecureBoot ISO' : 'ISO'
    case 'disk-image':
      if (asset.secureBoot) return t('final.disk_image.secure_boot')

      return asset.format
        ? t('final.disk_image.format', { format: asset.format })
        : t('final.disk_image.all')
    case 'pxe':
      return t(asset.secureBoot ? 'final.pxe.secure_boot' : 'final.pxe.boot')
    case 'board-image':
      return t('final.board_image')
    case 'kernel':
      return t('final.extra.kernel_image')
    case 'cmdline':
      return t('final.extra.cmdline')
    case 'initramfs':
      return t('final.extra.initramfs')
    case 'uki':
      return asset.secureBoot ? 'SecureBoot UKI' : 'UKI'
    default:
      return asset.kind
  }
})

const bareMetalDocs = '/platform-specific-installations/bare-metal-platforms'

// The guide that goes with an asset, if it has one.
const docs = computed(() => {
  if (asset.kind === 'iso' && asset.secureBoot) {
    return { label: t('final.disk_image.secure_boot_docs'), path: `${bareMetalDocs}/secureboot/` }
  }

  if (target !== 'metal') return undefined

  switch (asset.kind) {
    case 'iso':
      return { label: t('final.disk_image.iso_docs'), path: `${bareMetalDocs}/iso/` }
    case 'disk-image':
      return asset.secureBoot
        ? { label: t('final.disk_image.secure_boot_docs'), path: `${bareMetalDocs}/secureboot/` }
        : undefined
    case 'pxe':
      return { label: t('final.pxe.docs'), path: `${bareMetalDocs}/pxe/` }
    default:
      return undefined
  }
})
</script>

<template>
  <template v-if="asset.kind === 'pxe'">
    <dt class="mt-compact font-semibold text-content-emphasis">
      {{ label }}
      <!-- prettier-ignore -->
      <span v-if="docs">(<a :href="docsURL(version, docs.path)" target="_blank" rel="noopener noreferrer" class="font-medium text-accent-text underline">{{ docs.label }}</a>)</span>
    </dt>
    <dd class="mt-micro ps-base"><CodeBlock :code="asset.url" /></dd>
  </template>
  <template v-else>
    <dt class="mt-compact font-semibold text-content-emphasis">{{ label }}</dt>
    <dd class="mt-micro ps-base">
      <a :href="asset.url" class="font-medium text-accent-text underline">{{ asset.url }}</a>
      <template v-if="asset.checksums">
        <!-- prettier-ignore -->
        <a :href="asset.checksums.sha256" class="ml-tight type-control text-accent-text underline">(sha256)</a>
        <!-- prettier-ignore -->
        <a :href="asset.checksums.sha512" class="ml-micro type-control text-accent-text underline">(sha512)</a>
      </template>
      <!-- prettier-ignore -->
      <span v-else-if="!config.enterprise" class="ml-tight cursor-not-allowed type-meta text-content-secondary" :title="t('final.extra.checksum_enterprise_feature')">(checksums)</span>
      <!-- prettier-ignore -->
      <a v-if="asset.signature" :href="asset.signature" class="ml-tight type-control text-accent-text underline" :title="t('final.extra.signature_hint')">(sig)</a>
      <!-- prettier-ignore -->
      <span v-else-if="!config.enterprise" class="ml-tight cursor-not-allowed type-meta text-content-secondary" :title="t('final.extra.signature_enterprise_feature')">(signature)</span>
      <!-- prettier-ignore -->
      <span v-if="docs" class="ml-micro">(<a :href="docsURL(version, docs.path)" target="_blank" rel="noopener noreferrer" class="font-medium text-accent-text underline">{{ docs.label }}</a>)</span>
    </dd>
  </template>
</template>
