// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import createClient from 'openapi-fetch'

import type { components, paths } from '@/api/schema'

/**
 * Typed client for the Image Factory API, generated from api/openapi.yaml.
 * Run `npm run generate` after changing the OpenAPI contract.
 */
export const api = createClient<paths>()

export type Schemas = components['schemas']
