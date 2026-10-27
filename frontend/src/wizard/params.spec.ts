// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { dump } from 'js-yaml'
import { describe, expect, test } from 'vitest'
import type { LocationQuery } from 'vue-router'

import {
  clearFrom,
  currentStep,
  fromSchematic,
  parseQuery,
  previousStep,
  toQuery,
  toSchematic,
  type WizardParams,
} from '@/wizard/params'

function query(search: string): LocationQuery {
  const q: LocationQuery = {}

  for (const [key, value] of new URLSearchParams(search)) {
    const existing = q[key]

    q[key] =
      existing === undefined ? value : [...(Array.isArray(existing) ? existing : [existing]), value]
  }

  return q
}

const base: WizardParams = { secureBoot: false, cmdline: '', cmdlineSet: false, overlayOptions: '' }

describe('wizard URLs', () => {
  test('reads links to the final step', () => {
    expect(
      parseQuery(
        query(
          'target=metal&version=1.13.0&arch=amd64&platform=metal&secureboot=true&schematic-id=abc',
        ),
      ),
    ).toEqual({
      ...base,
      target: 'metal',
      version: '1.13.0',
      arch: 'amd64',
      secureBoot: true,
      schematicId: 'abc',
    })

    // Its intermediate steps carry every choice.
    expect(
      parseQuery(
        query(
          'target=sbc&version=1.13.0&board=rpi_generic&arch=arm64&extensions=siderolabs%2Fiscsi-tools&extensions=siderolabs%2Fgvisor&cmdline=console%3DttyS0&cmdline-set=true&overlay-options=configTxtAppend%3A+x',
        ),
      ),
    ).toEqual({
      ...base,
      target: 'sbc',
      version: '1.13.0',
      board: 'rpi_generic',
      arch: 'arm64',
      extensions: ['siderolabs/iscsi-tools', 'siderolabs/gvisor'],
      cmdline: 'console=ttyS0',
      cmdlineSet: true,
      overlayOptions: 'configTxtAppend: x',
    })
  })

  test('tells no extensions apart from extensions not chosen yet', () => {
    expect(parseQuery(query('extensions=-')).extensions).toEqual([])
    expect(parseQuery(query('')).extensions).toBeUndefined()
    expect(toQuery({ ...base, extensions: [] }).extensions).toEqual(['-'])
  })

  test('round-trips the choices', () => {
    const params: WizardParams = {
      ...base,
      target: 'cloud',
      version: '1.13.0',
      platform: 'aws',
      arch: 'arm64',
      secureBoot: true,
      extensions: ['siderolabs/gvisor'],
      cmdline: 'console=ttyS0',
      cmdlineSet: true,
      bootloader: 'sd-boot',
    }

    expect(
      parseQuery(query(new URLSearchParams(toQuery(params) as Record<string, string>).toString())),
    ).toEqual({ ...params, extensions: ['siderolabs/gvisor'] })
  })

  test('links the final step to the schematic only', () => {
    expect(
      toQuery({
        ...base,
        target: 'metal',
        version: '1.13.0',
        arch: 'amd64',
        extensions: ['siderolabs/gvisor'],
        cmdlineSet: true,
        schematicId: 'abc',
      }),
    ).toEqual({
      target: 'metal',
      version: '1.13.0',
      arch: 'amd64',
      platform: 'metal',
      'schematic-id': 'abc',
    })
  })
})

describe('wizard steps', () => {
  test('shows the first step with a choice left to make', () => {
    expect(currentStep(base)).toBe('target')
    expect(currentStep({ ...base, target: 'cloud' })).toBe('version')
    expect(currentStep({ ...base, target: 'cloud', version: '1.13.0' })).toBe('platform')
    expect(currentStep({ ...base, target: 'sbc', version: '1.13.0' })).toBe('board')
    expect(currentStep({ ...base, target: 'metal', version: '1.13.0' })).toBe('arch')
    expect(currentStep({ ...base, target: 'metal', version: '1.13.0', arch: 'amd64' })).toBe(
      'extensions',
    )
    expect(
      currentStep({ ...base, target: 'metal', version: '1.13.0', arch: 'amd64', extensions: [] }),
    ).toBe('customization')
    expect(
      currentStep({
        ...base,
        target: 'metal',
        version: '1.13.0',
        arch: 'amd64',
        extensions: [],
        cmdlineSet: true,
      }),
    ).toBe('final')
  })

  test('goes back to the previous step that asks something', () => {
    const cloud: WizardParams = {
      ...base,
      target: 'cloud',
      version: '1.13.0',
      platform: 'aws',
      arch: 'amd64',
    }

    expect(previousStep('extensions', cloud, false)).toBe('arch')
    // The platform decided the architecture.
    expect(previousStep('extensions', cloud, true)).toBe('platform')
    expect(previousStep('arch', { ...base, target: 'metal' }, false)).toBe('version')
    expect(previousStep('arch', cloud, false)).toBe('platform')
    expect(previousStep('target', base, false)).toBeUndefined()
  })

  test('going back clears the step and the ones after it', () => {
    const params: WizardParams = {
      ...base,
      target: 'cloud',
      version: '1.13.0',
      platform: 'aws',
      arch: 'amd64',
      secureBoot: true,
      extensions: [],
      cmdline: 'x',
      cmdlineSet: true,
      schematicId: 'abc',
    }

    expect(clearFrom('platform', params)).toEqual({ ...base, target: 'cloud', version: '1.13.0' })
    expect(clearFrom('customization', params)).toEqual({
      ...params,
      cmdline: '',
      cmdlineSet: false,
      schematicId: undefined,
    })
  })
})

describe('schematics', () => {
  test('builds the schematic of the choices', () => {
    expect(
      toSchematic(
        {
          ...base,
          target: 'sbc',
          extensions: ['siderolabs/iscsi-tools', 'siderolabs/gvisor'],
          cmdline: ' console=ttyS0   -console ',
          bootloader: 'sd-boot',
          overlayOptions: 'configTxtAppend: dtoverlay=x',
        },
        'kind: HostnameConfig',
        { name: 'rpi_generic', image: 'siderolabs/sbc-raspberrypi' },
      ),
    ).toEqual({
      overlay: {
        name: 'rpi_generic',
        image: 'siderolabs/sbc-raspberrypi',
        options: { configTxtAppend: 'dtoverlay=x' },
      },
      customization: {
        extraKernelArgs: ['console=ttyS0', '-console'],
        systemExtensions: { officialExtensions: ['siderolabs/gvisor', 'siderolabs/iscsi-tools'] },
        bootloader: 'sd-boot',
        embeddedMachineConfiguration: 'kind: HostnameConfig',
      },
    })
  })

  test('leaves out what the choices leave at their defaults', () => {
    expect(
      toSchematic({ ...base, target: 'metal', extensions: [], bootloader: 'auto' }, '', undefined),
    ).toEqual({
      customization: {},
    })
  })

  test('rejects overlay options that are not a mapping', () => {
    expect(() =>
      toSchematic({ ...base, target: 'sbc', overlayOptions: '- a' }, '', {
        name: 'rpi_generic',
        image: 'x',
      }),
    ).toThrow(/mapping/)
  })

  test('restores the choices from a schematic', () => {
    const schematic = dump({
      overlay: {
        image: 'siderolabs/sbc-raspberrypi',
        name: 'rpi_generic',
        options: { configTxtAppend: 'x' },
      },
      customization: {
        extraKernelArgs: ['console=ttyS0', '-console'],
        systemExtensions: { officialExtensions: ['siderolabs/gvisor'] },
        bootloader: 'dual-boot',
        embeddedMachineConfiguration: 'kind: HostnameConfig',
      },
    })

    expect(fromSchematic(schematic)).toEqual({
      params: {
        target: 'sbc',
        cmdline: 'console=ttyS0 -console',
        cmdlineSet: true,
        extensions: ['siderolabs/gvisor'],
        bootloader: 'dual-boot',
        overlayOptions: 'configTxtAppend: x',
      },
      embeddedConfig: 'kind: HostnameConfig',
      overlay: { name: 'rpi_generic', image: 'siderolabs/sbc-raspberrypi' },
    })

    expect(fromSchematic('customization: {}\n')).toEqual({
      params: { cmdline: '', cmdlineSet: true, extensions: [] },
      embeddedConfig: '',
    })
  })
})
