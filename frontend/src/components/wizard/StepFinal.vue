<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { Translation, useI18n } from 'vue-i18n'

import {
  assetsQuery,
  boardsQuery,
  featuresQuery,
  platformsQuery,
  schematicQuery,
} from '@/api/queries'
import AppSpinner from '@/components/AppSpinner.vue'
import CodeBlock from '@/components/CodeBlock.vue'
import ErrorNote from '@/components/ErrorNote.vue'
import AssetItem from '@/components/wizard/AssetItem.vue'
import ScanSummary from '@/components/wizard/ScanSummary.vue'
import WizardFooter from '@/components/WizardFooter.vue'
import { config } from '@/config'
import { docsURL, enterpriseDocsURL } from '@/lib/docs'
import { shortVersion } from '@/lib/versions'
import type { Arch, Target } from '@/wizard/params'

const props = defineProps<{
  schematicId: string
  target: Target
  version: string
  arch: Arch
  platform?: string
  board?: string
  secureBoot: boolean
}>()

const emit = defineEmits<{
  back: []
}>()

const { t } = useI18n()

const {
  data: schematic,
  isLoading: schematicLoading,
  error: schematicError,
} = useQuery(() => schematicQuery(props.schematicId))
const {
  data: features,
  isLoading: featuresLoading,
  error: featuresError,
} = useQuery(() => featuresQuery(props.version))
const {
  data: platforms,
  isLoading: platformsLoading,
  error: platformsError,
} = useQuery(() => ({
  ...platformsQuery(props.version),
  enabled: props.target === 'cloud',
}))
const {
  data: boards,
  isLoading: boardsLoading,
  error: boardsError,
} = useQuery(() => ({ ...boardsQuery(props.version), enabled: props.target === 'sbc' }))

const {
  data: assets,
  isLoading: assetsLoading,
  error: assetsError,
} = useQuery(() =>
  assetsQuery({
    schematic: props.schematicId,
    version: props.version,
    target: props.target,
    arch: props.arch,
    platform: props.platform,
    board: props.board,
    secureBoot: props.secureBoot,
  }),
)

const loading = computed(() =>
  [schematicLoading, featuresLoading, platformsLoading, boardsLoading, assetsLoading].some(
    (l) => l.value,
  ),
)

const error = computed(
  () =>
    [schematicError, featuresError, platformsError, boardsError, assetsError].find((e) => e.value)
      ?.value?.message,
)

const platformMeta = computed(() => platforms.value?.find((p) => p.name === props.platform))
const boardMeta = computed(() => boards.value?.find((b) => b.name === props.board))

const v = computed(() => `v${props.version}`)
</script>

<template>
  <div>
    <AppSpinner v-if="loading" class="size-8" />
    <ErrorNote v-else-if="error" :message="error" />
    <template v-else-if="assets && features">
      <ScanSummary v-if="features.includes('VEX')" :scan="assets.scan" />

      <h1 class="mb-snug type-section-title text-content-emphasis">{{ t('final.title') }}</h1>
      <div class="mb-base type-body">
        <Translation scope="global" keypath="final.image">
          <template #id><CodeBlock :code="schematicId" /></template>
        </Translation>
      </div>

      <div class="mb-base bg-surface-inset p-micro type-code-block whitespace-pre">
        {{ schematic }}
      </div>

      <div class="mb-base leading-[1.7142857] text-content-default">
        <h2 class="mb-snug type-section-title text-content-emphasis">
          {{ t('final.first_boot') }}
        </h2>
        <p v-if="target === 'metal'" class="mb-compact">{{ t('final.first_boot.metal') }}</p>
        <p v-else-if="target === 'cloud'" class="mb-compact">
          {{ t('final.first_boot.cloud', { platform: platformMeta?.label }) }}
        </p>
        <p v-else class="mb-compact">
          {{ t('final.first_boot.sbc', { board: boardMeta?.label }) }}
        </p>
        <dl class="mb-compact">
          <AssetItem
            v-for="asset in assets.bootAssets"
            :key="asset.url"
            :asset="asset"
            :target="target"
            :version="version"
          />
        </dl>

        <template
          v-if="target === 'metal' || (target === 'cloud' && platformMeta?.notOnlyDiskImage)"
        >
          <h2 class="mt-base mb-snug type-section-title text-content-emphasis">
            {{ t('final.initial_installation') }}
          </h2>
          <p class="mb-compact">
            {{ t('final.initial_installation.description') }}
          </p>
          <CodeBlock :code="assets.installerImage" />
        </template>

        <h2 class="mt-base mb-snug type-section-title text-content-emphasis">
          {{ t('final.upgrading') }}
        </h2>
        <p class="mb-compact">
          <Translation scope="global" keypath="final.upgrading.text">
            <template #link>
              <a
                :href="
                  docsURL(
                    version,
                    '/configure-your-talos-cluster/lifecycle-management/upgrading-talos/',
                  )
                "
                target="_blank"
                rel="noopener noreferrer"
                class="font-medium text-accent-text underline"
              >
                {{ t('final.upgrading.name') }}
              </a>
            </template>
          </Translation>
        </p>
        <CodeBlock :code="assets.installerImage" />

        <template v-if="features.includes('LocalTestCluster')">
          <h2 class="mt-base mb-snug type-section-title text-content-emphasis">
            {{ t('final.local_test_cluster') }}
          </h2>
          <p class="mb-compact">
            {{ t('final.local_test_cluster.prompt') }}
          </p>
          <CodeBlock
            :code="`talosctl cluster create qemu --schematic-id=${schematicId} --talos-version=${v}`"
          />
        </template>

        <h2 class="mt-base mb-snug type-section-title text-content-emphasis">
          {{ t('final.booter') }}
        </h2>
        <p class="mb-compact">
          <Translation scope="global" keypath="final.booter.text">
            <template #link>
              <!-- prettier-ignore -->
              <a href="https://github.com/siderolabs/booter" target="_blank" rel="noopener noreferrer" class="font-medium text-accent-text underline">booter</a>
            </template>
          </Translation>
        </p>
        <CodeBlock
          :code="`docker run --rm --network host ghcr.io/siderolabs/booter:v0.3.0 --talos-version=${v} --schematic-id=${schematicId}`"
        />

        <h2 class="mt-base mb-snug type-section-title text-content-emphasis">
          {{ t('final.documentation') }}
        </h2>
        <ul
          class="mb-compact list-disc ps-base *:my-micro *:ps-micro marker:text-content-secondary"
        >
          <li>
            <a
              target="_blank"
              rel="noopener noreferrer"
              :href="docsURL(version, `/getting-started/what's-new-in-talos/`)"
              class="font-medium text-accent-text underline"
            >
              {{ t('final.documentation.new', { version: shortVersion(version) }) }}
            </a>
          </li>
          <li>
            <a
              target="_blank"
              rel="noopener noreferrer"
              :href="docsURL(version, '/getting-started/support-matrix/')"
              class="font-medium text-accent-text underline"
            >
              {{ t('final.documentation.matrix', { version: shortVersion(version) }) }}
            </a>
          </li>
          <li>
            <a
              target="_blank"
              rel="noopener noreferrer"
              :href="docsURL(version, '/getting-started/getting-started/')"
              class="font-medium text-accent-text underline"
            >
              {{ t('final.documentation.getting_started') }}
            </a>
          </li>
          <li v-if="target === 'metal'">
            <a
              target="_blank"
              rel="noopener noreferrer"
              :href="
                docsURL(
                  version,
                  '/platform-specific-installations/bare-metal-platforms/network-config/',
                )
              "
              class="font-medium text-accent-text underline"
            >
              {{ t('final.documentation.metal') }}
            </a>
          </li>
          <li v-else-if="target === 'cloud' && platformMeta">
            <a
              target="_blank"
              rel="noopener noreferrer"
              :href="docsURL(version, platformMeta.documentation)"
              class="font-medium text-accent-text underline"
            >
              {{ t('final.documentation.cloud', { platform: platformMeta.label }) }}
            </a>
          </li>
          <li v-else-if="target === 'sbc' && boardMeta">
            <a
              target="_blank"
              rel="noopener noreferrer"
              :href="docsURL(version, boardMeta.documentation)"
              class="font-medium text-accent-text underline"
            >
              {{ t('final.documentation.sbc', { board: boardMeta.label }) }}
            </a>
          </li>
          <li v-if="secureBoot">
            <a
              target="_blank"
              rel="noopener noreferrer"
              :href="
                docsURL(
                  version,
                  '/platform-specific-installations/bare-metal-platforms/secureboot/',
                )
              "
              class="font-medium text-accent-text underline"
            >
              {{ t('final.documentation.secure_boot') }}
            </a>
          </li>
          <li v-if="features.includes('ProductionGuide')">
            <a
              target="_blank"
              rel="noopener noreferrer"
              :href="docsURL(version, '/getting-started/prodnotes/')"
              class="font-medium text-accent-text underline"
            >
              {{ t('final.documentation.production') }}
            </a>
          </li>
          <li v-if="features.includes('TroubleshootingGuide')">
            <a
              target="_blank"
              rel="noopener noreferrer"
              :href="docsURL(version, '/troubleshooting/troubleshooting/')"
              class="font-medium text-accent-text underline"
            >
              {{ t('final.documentation.troubleshooting') }}
            </a>
          </li>
        </ul>

        <template v-if="target === 'metal'">
          <h2 class="mt-base mb-snug type-section-title text-content-emphasis">
            {{ t('final.extra') }}
          </h2>
          <dl class="mb-compact">
            <AssetItem
              v-for="asset in assets.extraAssets"
              :key="asset.url"
              :asset="asset"
              :target="target"
              :version="version"
            />
            <!-- talosctl is listed with the parts of a regular boot image only. -->
            <template v-if="!secureBoot && assets.talosctl.length">
              <dt class="mt-compact font-semibold text-content-emphasis">
                {{ t('final.extra.talosctl') }}
              </dt>
              <dd v-for="binary in assets.talosctl" :key="binary.url" class="mt-micro ps-base">
                <a :href="binary.url" class="font-medium text-accent-text underline">
                  {{ binary.url }}
                </a>
              </dd>
            </template>
          </dl>
        </template>

        <template v-if="features.includes('SBOM')">
          <h2 class="mt-base mb-snug type-section-title text-content-emphasis">
            {{ t('final.extra.spdx') }}
          </h2>
          <dl class="mb-compact">
            <dd v-if="assets.sbom" class="mt-micro ps-base">
              <a :href="assets.sbom" class="font-medium text-accent-text underline">
                {{ assets.sbom }}
              </a>
            </dd>
            <dd v-else-if="!config.enterprise" class="mt-micro ps-base">
              <Translation scope="global" keypath="final.extra.spdx_enterprise_text">
                <template #link>
                  <a
                    :href="enterpriseDocsURL"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="font-medium text-accent-text underline"
                  >
                    {{ t('final.enterprise.learn_more') }}
                  </a>
                </template>
              </Translation>
            </dd>
          </dl>
        </template>

        <template v-if="features.includes('VEX')">
          <h2 class="mt-base mb-snug type-section-title text-content-emphasis">
            {{ t('final.extra.vex') }}
          </h2>
          <dl class="mb-compact">
            <dd v-if="assets.vex" class="mt-micro ps-base">
              <a :href="assets.vex" class="font-medium text-accent-text underline">
                {{ assets.vex }}
              </a>
            </dd>
            <dd v-else-if="!config.enterprise" class="mt-micro ps-base">
              <Translation scope="global" keypath="final.extra.vex_enterprise_text">
                <template #link>
                  <a
                    :href="enterpriseDocsURL"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="font-medium text-accent-text underline"
                  >
                    {{ t('final.enterprise.learn_more') }}
                  </a>
                </template>
              </Translation>
            </dd>
          </dl>
        </template>
      </div>
    </template>
    <WizardFooter back @back="emit('back')" />
  </div>
</template>
