// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { expect, test } from '@playwright/test'
import { milliseconds } from 'date-fns'

test('Sign in and out with Auth0', { tag: '@auth0' }, async ({ page }) => {
  // Not AUTH_USERNAME and AUTH_PASSWORD, which the other specs send as HTTP Basic credentials.
  const username = process.env.AUTH0_USERNAME ?? ''
  const password = process.env.AUTH0_PASSWORD ?? ''

  expect(username, 'AUTH0_USERNAME names the Auth0 test user').toBeTruthy()
  expect(password, 'AUTH0_PASSWORD is the Auth0 test user password').toBeTruthy()

  await test.step('See the landing page while signed out', async () => {
    await page.goto('/')
    await expect(
      page.getByRole('heading', { name: 'Talos Linux Image Factory Enterprise' }),
    ).toBeVisible()
    await page.getByRole('link', { name: 'Sign in' }).click()
  })

  // Auth0's own pages, as the test tenant shows them: the email, then the organization to sign in to, then the
  // password.
  await test.step('Log in with Auth0', async () => {
    await page.getByRole('textbox', { name: 'Email address' }).fill(username)
    await page.getByRole('button', { name: 'Continue' }).click()

    await page.getByRole('button', { name: 'Sidero Labs', exact: true }).click()

    await page.getByRole('textbox', { name: 'Password' }).fill(password)
    await page.getByRole('button', { name: 'Continue', exact: true }).click()
  })

  await test.step('Return to the wizard', async () => {
    await expect(page.getByRole('heading', { name: 'Hardware Type' })).toBeVisible({
      timeout: milliseconds({ seconds: 30 }),
    })
  })

  await test.step('Sign out', async () => {
    await page.goto('/logout')
    await expect(page.getByRole('heading', { name: 'Sign out' })).toBeVisible()
    await page.getByRole('button', { name: 'Sign out' }).click()

    // Auth0 ends its own session too, and sends the browser back to the factory, signed out.
    await expect(page.getByRole('link', { name: 'Sign in' })).toBeVisible({
      timeout: milliseconds({ seconds: 30 }),
    })
  })

  await test.step('Stay signed out, of Auth0 too', async () => {
    // With Auth0's session still alive, a deep link would sign straight back in.
    await page.goto('/ui/tokens')
    await expect(page.getByRole('textbox', { name: 'Email address' })).toBeVisible()
  })
})
