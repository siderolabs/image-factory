// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { expect, type Page } from '@playwright/test'
import { milliseconds } from 'date-fns'

import { TALOS_VERSION } from '../lib/constants'

/** The image wizard, from picking the hardware type to the schematic it creates. */
export class WizardPage {
  constructor(readonly page: Page) {}

  /** Starts the wizard, picks a target and the pinned Talos version, and ends on the version step. */
  async start(target: string) {
    await this.page.goto('/')
    await expect(this.page.getByRole('heading', { name: 'Hardware Type' })).toBeVisible()
    await this.page.getByRole('radio', { name: target }).check()
    await this.continueTo('Choose Talos Linux Version')
    await this.chooseVersion()
  }

  /**
   * Continues to the next step, and waits for it to be the one with this heading. Reaching the final step creates the
   * schematic and lists its downloads, which takes a while on a factory whose caches are still cold.
   */
  async continueTo(heading: string) {
    await this.page.getByRole('button', { name: 'Next' }).click()
    await expect(this.page.getByRole('heading', { name: heading })).toBeVisible({
      timeout: milliseconds({ seconds: 30 }),
    })
  }

  /** Goes back a step, and waits for it to be the one with this heading. */
  async backTo(heading: string) {
    await this.page.getByRole('button', { name: 'Back' }).click()
    await expect(this.page.getByRole('heading', { name: heading })).toBeVisible()
  }

  /** The ID of the schematic on the final step, which the URL links to. */
  async schematicID(): Promise<string> {
    await expect(this.page.getByRole('heading', { name: 'Schematic Ready' })).toBeVisible({
      timeout: milliseconds({ seconds: 30 }),
    })
    await expect(this.page).toHaveURL(/schematic-id=[0-9a-f]{64}/)

    return new URL(this.page.url()).searchParams.get('schematic-id') ?? ''
  }

  /** A download link of the final step, matched by the end of its URL. */
  download(path: string) {
    return this.page.getByRole('link', { name: new RegExp(`/${RegExp.escape(path)}$`) })
  }

  /**
   * Picks a Talos version on the version step. The version list is the select that offers it: its label isn't
   * reliable across UIs, and it can show up after the heading does.
   */
  private async chooseVersion(version = TALOS_VERSION) {
    await this.page
      .getByRole('combobox')
      .filter({ has: this.page.getByRole('option', { name: version, exact: true }) })
      .selectOption(version)
  }
}
