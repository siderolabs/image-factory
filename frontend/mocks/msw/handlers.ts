// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { faker } from '@faker-js/faker'
import { addDays, addMilliseconds } from 'date-fns'
import { dump } from 'js-yaml'
import { http, type HttpHandler, HttpResponse } from 'msw'

import type { Schemas } from '@/api/client'
import { parseGoDuration } from '@/lib/time'
import type { Arch, Target } from '@/wizard/params'

import {
  boards,
  extensions,
  fakeToken,
  features,
  platforms,
  scanReport,
  schematicAssets,
  tokenActors,
  vanillaSchematic,
  versions,
} from '../fixtures'
import { apiPath } from './helpers'

async function sha256(text: string) {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text))

  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('')
}

/**
 * Handlers that answer every API request the frontend makes, like a factory serving the data of mocks/fixtures.ts.
 * Schematics and tokens are kept in memory, so that what is created can be read back; each call starts from a fresh
 * state.
 *
 * Storybook and the unit tests install these by default, so stories and tests only override what they need, with
 * `msw.use()` or `worker.use()`, such as with apiError() or a handler for apiPath().
 */
export function apiHandlers(): HttpHandler[] {
  const schematics = new Map([[vanillaSchematic.id, vanillaSchematic.schematic]])
  const tokens = faker.helpers.multiple(fakeToken, { count: 3 })

  return [
    http.get(apiPath('/versions'), () => HttpResponse.json<Schemas['VersionList']>(versions)),

    http.get(apiPath('/api/version/{version}/platforms'), () =>
      HttpResponse.json<Schemas['Platform'][]>(platforms),
    ),

    http.get(apiPath('/api/version/{version}/boards'), () =>
      HttpResponse.json<Schemas['Board'][]>(boards),
    ),

    http.get(apiPath('/api/version/{version}/features'), () =>
      HttpResponse.json<Schemas['VersionFeatures']>(features),
    ),

    http.get(apiPath('/version/{version}/extensions/official'), () =>
      HttpResponse.json<Schemas['Extension'][]>(extensions),
    ),

    http.post(apiPath('/schematics'), async ({ request }) => {
      const schematic = dump(await request.json(), { sortKeys: true })
      const id = await sha256(schematic)

      schematics.set(id, schematic)

      return HttpResponse.json<Schemas['SchematicCreated']>({ id, schematic }, { status: 201 })
    }),

    http.get<{ schematic: string }>(apiPath('/schematics/{schematic}'), ({ params }) => {
      const schematic = schematics.get(params.schematic)

      return schematic
        ? HttpResponse.text(schematic, { headers: { 'Content-Type': 'application/yaml' } })
        : HttpResponse.text('schematic not found', { status: 404 })
    }),

    http.get<{ schematic: string; version: string }>(
      apiPath('/api/schematics/{schematic}/{version}/assets'),
      ({ params, request }) => {
        const query = new URL(request.url).searchParams

        return HttpResponse.json<Schemas['SchematicAssets']>(
          schematicAssets({
            schematic: params.schematic,
            version: params.version.replace(/^v/, ''),
            target: query.get('target') as Target,
            arch: query.get('arch') as Arch,
            platform: query.get('platform') ?? undefined,
            board: query.get('board') ?? undefined,
            secureBoot: query.get('secureboot') === 'true',
          }),
        )
      },
    ),

    http.get(apiPath('/scans/{schematic}/{version}/{arch}/{report}'), () =>
      HttpResponse.json(scanReport),
    ),

    http.get(apiPath('/api/tokens/actors'), () =>
      HttpResponse.json<Schemas['APITokenActor'][]>(tokenActors),
    ),

    http.get(apiPath('/tokens'), () => HttpResponse.json<Schemas['APITokenList']>({ tokens })),

    http.post(apiPath('/tokens'), async ({ request }) => {
      const body = (await request.json()) as Schemas['APITokenCreateRequest']
      const actor = tokenActors.find((a) => a.id === body.actor)
      const created = new Date()
      const ttl = body.ttl ? parseGoDuration(body.ttl) : undefined

      const token: Schemas['APIToken'] = {
        id: faker.string.ulid({ refDate: created }),
        name: body.name || faker.string.alphanumeric(8),
        created_at: created.toISOString(),
        expires_at: (ttl ? addMilliseconds(created, ttl) : addDays(created, 90)).toISOString(),
        scopes: body.scopes ?? actor?.scopes ?? [],
        issuable_scopes: body.issuable_scopes ?? actor?.issuable_scopes,
      }

      if (body.stored) tokens.push(token)

      return HttpResponse.json<Schemas['APITokenCreated']>({
        ...token,
        // Tokens are ES256-signed JWTs.
        token: [
          'eyJhbGciOiJFUzI1NiJ9',
          faker.string.alphanumeric(180),
          faker.string.alphanumeric(86),
        ].join('.'),
        org_id: faker.string.uuid(),
        stored: !!body.stored,
      })
    }),

    http.post<{ id: string }>(apiPath('/tokens/{id}/revoke'), ({ params }) => {
      const index = tokens.findIndex((t) => t.id === params.id)

      if (index === -1) return HttpResponse.text('token not found', { status: 404 })

      tokens.splice(index, 1)

      return new HttpResponse(null, { status: 204 })
    }),
  ]
}
