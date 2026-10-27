<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->
<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { load } from 'js-yaml'
import lt from 'semver/functions/lt'
import { computed, ref, useId } from 'vue'
import { Translation, useI18n } from 'vue-i18n'

import { boardsQuery, featuresQuery } from '@/api/queries'
import AppInput from '@/components/AppInput.vue'
import AppSpinner from '@/components/AppSpinner.vue'
import AppTextarea from '@/components/AppTextarea.vue'
import ErrorNote from '@/components/ErrorNote.vue'
import InlineCode from '@/components/InlineCode.vue'
import RadioCard from '@/components/RadioCard.vue'
import WizardFooter from '@/components/WizardFooter.vue'
import { docsURL } from '@/lib/docs'
import { draft } from '@/wizard/draft'
import type { Target, WizardParams } from '@/wizard/params'

const { target, version, board, secureBoot } = defineProps<{
  target: Target
  version: string
  board?: string
  secureBoot: boolean
}>()

const emit = defineEmits<{
  back: []
  next: [patch: Partial<WizardParams>]
}>()

const { t } = useI18n()
const id = useId()

const {
  data: features,
  isLoading: featuresLoading,
  error: featuresError,
} = useQuery(() => featuresQuery(version))
const {
  data: boards,
  isLoading: boardsLoading,
  error: boardsError,
} = useQuery(() => ({ ...boardsQuery(version), enabled: target === 'sbc' }))

const overlayImage = computed(() => boards.value?.find((b) => b.name === board)?.overlay.image)

const cmdline = ref(draft.value.cmdline ?? '')
const embeddedConfig = ref(draft.value.embeddedConfig ?? '')
const overlayOptions = ref(draft.value.overlayOptions ?? '')
// GRUB can't boot SecureBoot images, so it isn't offered with SecureBoot. Only the starting choice depends on
// secureBoot, and the step is recreated whenever the earlier choices change.
// eslint-disable-next-line vue/no-setup-props-reactivity-loss
const bootloader = ref(
  secureBoot && draft.value.bootloader === 'grub' ? 'auto' : (draft.value.bootloader ?? 'auto'),
)

const overlayError = ref<string>()

const loading = computed(() => featuresLoading.value || boardsLoading.value)
const error = computed(() => (featuresError.value ?? boardsError.value)?.message)

function next() {
  overlayError.value = undefined

  if (target === 'sbc' && overlayOptions.value.trim()) {
    try {
      const options = load(overlayOptions.value)

      if (options === null || typeof options !== 'object' || Array.isArray(options)) {
        throw new Error('overlay options must be a YAML mapping')
      }
    } catch (err) {
      overlayError.value = err instanceof Error ? err.message : String(err)

      return
    }
  }

  // Kept out of the URL: the final step reads it from here.
  draft.value.embeddedConfig = features.value?.includes('EmbeddedConfig')
    ? embeddedConfig.value
    : ''

  emit('next', {
    cmdline: cmdline.value.trim(),
    cmdlineSet: true,
    overlayOptions: target === 'sbc' ? overlayOptions.value.trim() : '',
    bootloader: features.value?.includes('BootloaderSelection') ? bootloader.value : undefined,
  })
}
</script>

<template>
  <form @submit.prevent="next">
    <h1 class="mb-snug type-section-title text-content-emphasis">{{ t('customization.title') }}</h1>
    <AppSpinner v-if="loading" class="size-8" />
    <ErrorNote v-else-if="error" :message="error" />
    <div v-else class="mb-base">
      <div class="mb-base">
        <label :for="`${id}-cmdline`" class="mb-tight block type-label text-content-secondary">
          {{ t('customization.cmdline') }}
        </label>
        <AppInput :id="`${id}-cmdline`" v-model="cmdline" placeholder="-console console=tty0" />
      </div>

      <div class="ms-tight mb-base max-w-xl leading-[1.7142857] text-content-default">
        <p class="mb-compact">
          <Translation scope="global" keypath="customization.cmdline.description_text">
            <template #link>
              <a
                :href="docsURL(version, '/reference/kernel/')"
                target="_blank"
                rel="noopener noreferrer"
                class="font-medium text-accent-text underline"
              >
                {{ t('customization.cmdline.arguments') }}
              </a>
            </template>
          </Translation>
        </p>
        <p class="mb-compact">
          <Translation scope="global" keypath="customization.cmdline.example_text">
            <template #example>
              <InlineCode>console=ttyS0,115200</InlineCode>
            </template>
            <template #dash>
              <InlineCode>-</InlineCode>
            </template>
            <template #remove>
              <InlineCode>-console</InlineCode>
            </template>
          </Translation>
        </p>
        <p v-if="secureBoot" class="mb-compact">{{ t('customization.cmdline.secure_boot') }}</p>
        <p v-else-if="lt(version, '1.10.0')" class="mb-compact">
          <Translation scope="global" keypath="customization.cmdline.note_text">
            <template #installer>
              <InlineCode>installer</InlineCode>
            </template>
            <template #field>
              <InlineCode>machine.install.extraKernelArgs</InlineCode>
            </template>
          </Translation>
        </p>
        <p>{{ t('customization.cmdline.skip') }}</p>
      </div>

      <template v-if="features?.includes('EmbeddedConfig')">
        <div class="mb-base">
          <label :for="`${id}-embedded`" class="mb-tight block type-label text-content-secondary">
            {{ t('customization.embedded_config') }}
          </label>
          <AppTextarea
            :id="`${id}-embedded`"
            v-model="embeddedConfig"
            class="h-16 min-h-24"
            placeholder="apiVersion: v1alpha1
kind: HostnameConfig
hostname: my-custom-hostname
auto: off"
          />
        </div>

        <div class="ms-tight mb-base max-w-xl leading-[1.7142857] text-content-default">
          <p class="mb-compact">
            <Translation scope="global" keypath="customization.embedded_config.description_text">
              <template #link>
                <a
                  :href="
                    docsURL(
                      version,
                      '/configure-your-talos-cluster/system-configuration/acquire#embedded-configuration',
                    )
                  "
                  target="_blank"
                  rel="noopener noreferrer"
                  class="font-medium text-accent-text underline"
                >
                  {{ t('customization.embedded_config.reference') }}
                </a>
              </template>
            </Translation>
          </p>
          <p>{{ t('customization.embedded_config.skip') }}</p>
        </div>
      </template>

      <template v-if="target === 'sbc'">
        <div class="mt-base mb-base">
          <label :for="`${id}-overlay`" class="mb-tight block type-label text-content-secondary">
            {{ t('customization.overlay') }}
          </label>
          <AppTextarea
            :id="`${id}-overlay`"
            v-model="overlayOptions"
            class="h-16"
            placeholder="configTxtAppend: 'dtoverlay=vc4-fkms-v3d'"
          />
        </div>
        <ErrorNote v-if="overlayError" :message="overlayError" />

        <div class="ms-tight max-w-xl leading-[1.7142857] text-content-default">
          <p class="mb-compact">
            <Translation scope="global" keypath="customization.overlay.description_text">
              <template #link>
                <a
                  :href="`https://github.com/${overlayImage}`"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="font-medium text-accent-text underline"
                >
                  {{ t('customization.overlay.name', { image: overlayImage }) }}
                </a>
              </template>
            </Translation>
          </p>
          <p class="mb-compact">{{ t('customization.overlay.syntax') }}</p>
          <p>{{ t('customization.overlay.skip') }}</p>
        </div>
      </template>

      <div v-if="features?.includes('BootloaderSelection')" class="mt-base mb-base space-y-tight">
        <p class="mb-tight block type-label text-content-secondary">
          {{ t('customization.bootloader.title') }}
        </p>
        <RadioCard
          v-model="bootloader"
          name="bootloader"
          value="auto"
          :label="t('customization.bootloader.auto.name')"
        >
          {{ t('customization.bootloader.auto.description') }}
        </RadioCard>
        <RadioCard
          v-model="bootloader"
          name="bootloader"
          value="sd-boot"
          :label="t('customization.bootloader.sd_boot.name')"
        >
          {{ t('customization.bootloader.sd_boot.description') }}
        </RadioCard>
        <RadioCard
          v-if="!secureBoot"
          v-model="bootloader"
          name="bootloader"
          value="grub"
          :label="t('customization.bootloader.grub.name')"
        >
          {{ t('customization.bootloader.grub.description') }}
        </RadioCard>
        <RadioCard
          v-model="bootloader"
          name="bootloader"
          value="dual-boot"
          :label="t('customization.bootloader.dual_boot.name')"
        >
          {{ t('customization.bootloader.dual_boot.description') }}
        </RadioCard>
      </div>
    </div>
    <WizardFooter back :next="!loading && !error" @back="emit('back')" />
  </form>
</template>
