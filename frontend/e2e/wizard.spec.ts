// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { expect, test } from '@playwright/test'

import { TALOS_VERSION } from './lib/constants'
import { WizardPage } from './models/WizardPage'

const version = `v${TALOS_VERSION}`

test.describe.configure({ mode: 'parallel' })

test('Create a bare-metal image with an extension and kernel arguments', async ({ page }) => {
  const wizard = new WizardPage(page)

  await test.step('Choose the target and version', async () => {
    await wizard.start('Bare-metal Machine')
    await wizard.continueTo('Machine Architecture')
  })

  await test.step('Keep the default architecture', async () => {
    await expect(page.getByRole('radio', { name: /^amd64/ })).toBeChecked()
    await wizard.continueTo('System Extensions')
  })

  await test.step('Pick an extension', async () => {
    await page.getByRole('searchbox', { name: 'Search' }).fill('gvisor')
    await page.getByRole('checkbox', { name: /^siderolabs\/gvisor \(/ }).check()
    await wizard.continueTo('Customization')
  })

  await test.step('Add kernel arguments', async () => {
    await page
      .getByRole('textbox', { name: 'Extra kernel command line arguments:' })
      .fill('console=ttyS0')
    await wizard.continueTo('Schematic Ready')
  })

  await test.step('Check the schematic and its downloads', async () => {
    const id = await wizard.schematicID()

    await expect(page.getByText('siderolabs/gvisor').first()).toBeVisible()
    await expect(page.getByText('console=ttyS0').first()).toBeVisible()

    for (const asset of ['metal-amd64.iso', 'metal-amd64.raw.zst', 'metal-amd64.qcow2']) {
      await expect(wizard.download(`image/${id}/${version}/${asset}`)).toBeVisible()
    }

    await expect(page.getByText(`/pxe/${id}/${version}/metal-amd64`)).toBeVisible()
    await expect(page.getByText(`metal-installer/${id}:${version}`).first()).toBeVisible()
  })
})

test(
  'Link the checksums and signature of a download',
  { tag: '@enterprise-factory' },
  async ({ page }) => {
    const wizard = new WizardPage(page)

    await wizard.start('Bare-metal Machine')
    await wizard.continueTo('Machine Architecture')
    await wizard.continueTo('System Extensions')
    await wizard.continueTo('Customization')
    await wizard.continueTo('Schematic Ready')
    const iso = RegExp.escape(`image/${await wizard.schematicID()}/${version}/metal-amd64.iso`)

    // The ISO is the first download, so its checksums and signature are the first ones listed.
    for (const [name, suffix] of [
      ['(sha256)', '.sha256'],
      ['(sha512)', '.sha512'],
      ['(sig)', '.sigstore.json'],
    ]) {
      await expect(page.getByRole('link', { name, exact: true }).first()).toHaveAttribute(
        'href',
        new RegExp(`/${iso}${RegExp.escape(suffix)}$`),
      )
    }
  },
)

test('Create a SecureBoot bare-metal image', async ({ page }) => {
  const wizard = new WizardPage(page)

  await wizard.start('Bare-metal Machine')
  await wizard.continueTo('Machine Architecture')

  // The checkbox is drawn as a switch that covers it, so click the switch's label, as a user would.
  const secureBoot = page.getByRole('checkbox', { name: 'SecureBoot' })

  await page.locator('label').filter({ has: secureBoot }).click()
  await expect(secureBoot).toBeChecked()
  await wizard.continueTo('System Extensions')
  await wizard.continueTo('Customization')
  await wizard.continueTo('Schematic Ready')
  const id = await wizard.schematicID()

  for (const asset of [
    'metal-amd64-secureboot.iso',
    'metal-amd64-secureboot.raw.zst',
    'metal-amd64-secureboot-uki.efi',
  ]) {
    await expect(wizard.download(`image/${id}/${version}/${asset}`)).toBeVisible()
  }

  await expect(page.getByText(`metal-installer-secureboot/${id}:${version}`).first()).toBeVisible()
})

test('Create a cloud image', async ({ page }) => {
  const wizard = new WizardPage(page)

  await wizard.start('Cloud Server')
  await wizard.continueTo('Cloud')

  await expect(page.getByRole('radio', { name: /^Amazon Web Services \(AWS\)/ })).toBeChecked()
  await wizard.continueTo('Machine Architecture')
  await wizard.continueTo('System Extensions')
  await wizard.continueTo('Customization')
  await wizard.continueTo('Schematic Ready')
  const id = await wizard.schematicID()

  await expect(wizard.download(`image/${id}/${version}/aws-amd64.raw.xz`)).toBeVisible()
  await expect(page.getByText(`aws-installer/${id}:${version}`).first()).toBeVisible()
})

test('Skip the architecture step for a single-architecture platform', async ({ page }) => {
  const wizard = new WizardPage(page)

  await wizard.start('Cloud Server')
  await wizard.continueTo('Cloud')

  await page.getByRole('radio', { name: /^Digital Ocean/ }).check()

  await test.step('Go past the architecture step, which the platform decides', async () => {
    await wizard.continueTo('System Extensions')
  })

  await test.step('Go back past it too', async () => {
    await wizard.backTo('Cloud')
    await expect(page.getByRole('radio', { name: /^Digital Ocean/ })).toBeChecked()
  })

  await wizard.continueTo('System Extensions')
  await wizard.continueTo('Customization')
  await wizard.continueTo('Schematic Ready')
  const id = await wizard.schematicID()

  await expect(wizard.download(`image/${id}/${version}/digital-ocean-amd64.raw.gz`)).toBeVisible()
})

test('Create a single-board computer image with overlay options', async ({ page }) => {
  const wizard = new WizardPage(page)

  const overlayOptions = "configTxtAppend: 'dtoverlay=vc4-fkms-v3d'"

  await wizard.start('Single Board Computer')
  await wizard.continueTo('Single Board Computer')

  await test.step('Keep the default board', async () => {
    await expect(page.getByRole('radio', { name: /^Raspberry Pi Series/ })).toBeChecked()
    await wizard.continueTo('System Extensions')
    await wizard.continueTo('Customization')
  })

  await test.step('Add overlay options', async () => {
    await page
      .getByRole('textbox', { name: 'Extra overlay options (advanced):' })
      .fill(overlayOptions)
    await wizard.continueTo('Schematic Ready')
  })

  const id = await wizard.schematicID()

  await test.step('Check the schematic and its download', async () => {
    await expect(page.getByText('siderolabs/sbc-raspberrypi').first()).toBeVisible()
    await expect(page.getByText('dtoverlay=vc4-fkms-v3d').first()).toBeVisible()
    await expect(wizard.download(`image/${id}/${version}/metal-arm64.raw.xz`)).toBeVisible()
  })

  await test.step('Reload the schematic link', async () => {
    await page.reload()
    expect(await wizard.schematicID()).toBe(id)
  })

  await test.step('Go back to the customization the schematic records', async () => {
    await wizard.backTo('Customization')
    await expect(
      page.getByRole('textbox', { name: 'Extra overlay options (advanced):' }),
    ).toHaveValue(/dtoverlay=vc4-fkms-v3d/)
  })
})

test('Keep the earlier choice when going back to a step', async ({ page }) => {
  const wizard = new WizardPage(page)

  await wizard.start('Cloud Server')
  await wizard.continueTo('Cloud')

  await page.getByRole('radio', { name: /^Google Cloud \(GCP\)/ }).check()
  await wizard.continueTo('Machine Architecture')
  await wizard.backTo('Cloud')

  await expect(page.getByRole('radio', { name: /^Google Cloud \(GCP\)/ })).toBeChecked()
})

test('Switch the language', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Hardware Type' })).toBeVisible()

  // The factory remembers the language once a request with ?lang= reaches it, which may land after the page has
  // switched, so wait for it before reloading.
  const saved = page.waitForResponse(
    (response) => response.url().includes('lang=fr') && response.ok(),
  )

  await page
    .getByRole('combobox')
    .filter({ has: page.getByRole('option', { name: 'Français' }) })
    .selectOption('fr')
  await expect(page.getByRole('heading', { name: 'Type de matériel' })).toBeVisible()
  await saved

  await test.step('The language sticks after a reload', async () => {
    await page.reload()
    await expect(page.getByRole('heading', { name: 'Type de matériel' })).toBeVisible()
  })
})
