// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { expect, test } from '@playwright/test'

test(
  'Create and revoke an API token',
  { tag: '@enterprise-factory' },
  async ({ page }, testInfo) => {
    // Every browser runs against the same factory, so each token gets a name of its own.
    const name = `e2e ${testInfo.project.name} ${Date.now()}`

    await page.goto('/ui/tokens')
    await expect(page.getByRole('heading', { name: 'API Tokens' })).toBeVisible()

    await test.step('Create a token', async () => {
      await page.getByRole('button', { name: 'Create Token' }).click()
      await expect(page.getByRole('heading', { name: 'Create Token' })).toBeVisible()

      const nameField = page.getByRole('textbox', { name: 'Name' })

      await nameField.fill(name)
      await nameField.press('Enter')
    })

    await test.step('See its secret, with the machine config patch for pulling images', async () => {
      await expect(page.getByRole('heading', { name: 'Token Created' })).toBeVisible()
      await expect(page.getByText('kind: RegistryAuthConfig')).toBeVisible()
      await page.getByRole('button', { name: 'Close' }).click()
    })

    const row = page.getByRole('row').filter({ hasText: name })

    await expect(row).toBeVisible()

    await test.step('Revoke it', async () => {
      await row.getByRole('button', { name: 'Revoke' }).click()
      await expect(page.getByRole('heading', { name: 'Revoke Token' })).toBeVisible()

      // The row and the confirmation both have a Revoke button; the confirmation's comes last.
      await page.getByRole('button', { name: 'Revoke', exact: true }).last().click()
      await expect(row).toBeHidden()
    })
  },
)
