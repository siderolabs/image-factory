// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { dump, load } from 'js-yaml'
import type { LocationQuery, LocationQueryRaw } from 'vue-router'

import type { Schemas } from '@/api/client'

export type Target = 'metal' | 'cloud' | 'sbc'

export type Arch = 'amd64' | 'arm64'

/**
 * The choices made in the wizard so far. They live in the URL query, so that bookmarks and shared links keep working.
 */
export interface WizardParams {
  target?: Target
  /** Talos version without the leading v, such as 1.13.0. */
  version?: string
  /** Cloud platform, for the cloud target. */
  platform?: string
  /** Single-board computer, for the sbc target. */
  board?: string
  arch?: Arch
  secureBoot: boolean
  /** Official extensions; empty once the extensions step is done without picking any, undefined before. */
  extensions?: string[]
  cmdline: string
  /** Whether the customization step is done. */
  cmdlineSet: boolean
  bootloader?: string
  overlayOptions: string
  /** The created schematic, on the final step. */
  schematicId?: string
}

/** The wizard steps, in order. */
export const steps = [
  'target',
  'version',
  'platform',
  'board',
  'arch',
  'extensions',
  'customization',
  'final',
] as const

export type Step = (typeof steps)[number]

// "-" stands for "no extensions", to tell it apart from "not chosen yet".
const noExtensions = '-'

function first(value: LocationQuery[string]): string | undefined {
  const v = Array.isArray(value) ? value[0] : value

  return v ?? undefined
}

function all(value: LocationQuery[string]): string[] {
  return (Array.isArray(value) ? value : [value]).filter((v): v is string => !!v)
}

function oneOf<T extends string>(value: string | undefined, choices: readonly T[]): T | undefined {
  return choices.find((c) => c === value)
}

/** Reads the wizard choices from the URL query. */
export function parseQuery(query: LocationQuery): WizardParams {
  const target = oneOf(first(query.target), ['metal', 'cloud', 'sbc'] as const)
  const extensions = all(query.extensions)

  return {
    target,
    version: first(query.version)?.replace(/^v/, '') || undefined,
    // The metal target also has platform=metal in its query.
    platform: target === 'cloud' ? first(query.platform) || undefined : undefined,
    board: target === 'sbc' ? first(query.board) || undefined : undefined,
    arch: oneOf(first(query.arch), ['amd64', 'arm64'] as const),
    secureBoot: first(query.secureboot) === 'true',
    extensions: extensions.length ? extensions.filter((e) => e !== noExtensions) : undefined,
    cmdline: first(query.cmdline)?.trim() ?? '',
    cmdlineSet: !!first(query['cmdline-set']),
    bootloader: first(query.bootloader) || undefined,
    overlayOptions: first(query['overlay-options'])?.trim() ?? '',
    schematicId: first(query['schematic-id']) || undefined,
  }
}

/**
 * Writes the wizard choices to a URL query. The final step only keeps what the schematic doesn't hold: the rest
 * comes back from the schematic.
 */
export function toQuery(params: WizardParams): LocationQueryRaw {
  const query: LocationQueryRaw = {}

  if (params.target) query.target = params.target
  if (params.version) query.version = params.version
  if (params.arch) query.arch = params.arch
  if (params.target === 'metal') query.platform = 'metal'
  if (params.target === 'cloud' && params.platform) query.platform = params.platform
  if (params.secureBoot) query.secureboot = 'true'

  if (params.schematicId) {
    query['schematic-id'] = params.schematicId

    return query
  }

  if (params.target === 'sbc' && params.board) query.board = params.board
  if (params.bootloader) query.bootloader = params.bootloader
  if (params.extensions)
    query.extensions = params.extensions.length ? params.extensions : [noExtensions]
  if (params.cmdline) query.cmdline = params.cmdline
  if (params.cmdlineSet) query['cmdline-set'] = 'true'
  // The embedded machine configuration is never put in the URL, to keep secrets out of the browser history.
  if (params.overlayOptions) query['overlay-options'] = params.overlayOptions

  return query
}

/** Whether a step asks anything for the target chosen. */
function stepApplies(step: Step, params: WizardParams): boolean {
  switch (step) {
    case 'platform':
      return params.target === 'cloud'
    case 'board':
      return params.target === 'sbc'
    default:
      return true
  }
}

/** The step to show: the first one that still has a choice to make. */
export function currentStep(params: WizardParams): Step {
  if (!params.target) return 'target'
  if (!params.version) return 'version'
  if (params.target === 'cloud' && !params.platform) return 'platform'
  if (params.target === 'sbc' && !params.board) return 'board'
  if (!params.arch) return 'arch'
  if (!params.extensions) return 'extensions'
  if (!params.cmdlineSet) return 'customization'

  return 'final'
}

/** Returns the choices without those made on a step and after it, to go back to that step. */
export function clearFrom(step: Step, params: WizardParams): WizardParams {
  const index = steps.indexOf(step)
  const cleared: WizardParams = { ...params, schematicId: undefined }

  const clears = (s: Step) => steps.indexOf(s) >= index

  if (clears('target')) cleared.target = undefined
  if (clears('version')) cleared.version = undefined
  if (clears('platform')) cleared.platform = undefined
  if (clears('board')) cleared.board = undefined

  if (clears('arch')) {
    cleared.arch = undefined
    cleared.secureBoot = false
  }

  if (clears('extensions')) cleared.extensions = undefined

  if (clears('customization')) {
    cleared.cmdline = ''
    cleared.cmdlineSet = false
    cleared.bootloader = undefined
    cleared.overlayOptions = ''
  }

  return cleared
}

/**
 * The step that Back returns to from a step: the previous one that asks something. The architecture step is
 * skipped when the platform or board decided the architecture.
 */
export function previousStep(
  step: Step,
  params: WizardParams,
  archDecided: boolean,
): Step | undefined {
  for (let i = steps.indexOf(step) - 1; i >= 0; i--) {
    const s = steps[i]

    if (!stepApplies(s, params)) continue
    if (s === 'arch' && archDecided) continue

    return s
  }

  return undefined
}

/** The overlay of a board, as a schematic names it. */
export type Overlay = Schemas['Board']['overlay']

/** Builds the schematic of the choices. */
export function toSchematic(
  params: WizardParams,
  embeddedConfig: string,
  overlay: Overlay | undefined,
): Schemas['Schematic'] {
  // The generated type requires bootloader, as the contract gives it a default; the server doesn't require it.
  const customization: Partial<Schemas['Customization']> = {}

  const args = params.cmdline.split(/\s+/).filter(Boolean)
  if (args.length) customization.extraKernelArgs = args

  if (params.extensions?.length) {
    customization.systemExtensions = { officialExtensions: [...params.extensions].sort() }
  }

  if (params.bootloader && params.bootloader !== 'auto') {
    customization.bootloader = params.bootloader as Schemas['Customization']['bootloader']
  }

  if (embeddedConfig.trim()) customization.embeddedMachineConfiguration = embeddedConfig

  const schematic = { customization } as Schemas['Schematic']

  if (params.target === 'sbc' && overlay) {
    schematic.overlay = { name: overlay.name, image: overlay.image }

    if (params.overlayOptions) {
      const options = load(params.overlayOptions)

      if (options === null || typeof options !== 'object' || Array.isArray(options)) {
        throw new Error('overlay options must be a YAML mapping')
      }

      schematic.overlay.options = options as Record<string, unknown>
    }
  }

  return schematic
}

/** The choices that a schematic records, to restore a wizard from a schematic link. */
export interface SchematicChoices {
  params: Partial<WizardParams>
  embeddedConfig: string
  overlay?: Overlay
}

/** Reads the choices back from a canonical schematic in YAML. */
export function fromSchematic(schematicYAML: string): SchematicChoices {
  const schematic = (load(schematicYAML) ?? {}) as Schemas['Schematic']
  const customization: Partial<Schemas['Customization']> = schematic.customization ?? {}

  const params: Partial<WizardParams> = {
    cmdline: (customization.extraKernelArgs ?? []).join(' '),
    cmdlineSet: true,
    extensions: customization.systemExtensions?.officialExtensions ?? [],
  }

  // "none" is also what an unset or "auto" choice comes back as.
  if (customization.bootloader && (customization.bootloader as string) !== 'none') {
    params.bootloader = customization.bootloader
  }

  const choices: SchematicChoices = {
    params,
    embeddedConfig: customization.embeddedMachineConfiguration ?? '',
  }

  const overlay = schematic.overlay

  if (overlay && (overlay.name || overlay.image || overlay.options)) {
    params.target = 'sbc'
    choices.overlay = { name: overlay.name ?? '', image: overlay.image ?? '' }

    if (overlay.options && Object.keys(overlay.options).length) {
      params.overlayOptions = dump(overlay.options).trimEnd()
    }
  }

  return choices
}
