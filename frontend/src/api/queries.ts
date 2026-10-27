// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import {
  type DefaultError,
  mutationOptions,
  type QueryKey,
  queryOptions,
  type UndefinedInitialQueryOptions,
} from '@tanstack/vue-query'

import { api, type Schemas } from '@/api/client'
import type { Arch, Target } from '@/wizard/params'
import type { ScanReport } from '@/wizard/scan'

/** queryOptions for what doesn't change while the page is open, such as what a Talos version supports. */
function staticQueryOptions<
  TQueryFnData,
  TError = DefaultError,
  TData = TQueryFnData,
  TQueryKey extends QueryKey = QueryKey,
>(options: UndefinedInitialQueryOptions<TQueryFnData, TError, TData, TQueryKey>) {
  return queryOptions<TQueryFnData, TError, TData, TQueryKey>({ staleTime: Infinity, ...options })
}

/** The Talos versions that can build images for a target. */
export function versionsQuery(target: Target) {
  return staticQueryOptions({
    queryKey: ['versions', target],
    queryFn: () => api.GET('/versions', { params: { query: { target } } }),
  })
}

/** The cloud platforms that a Talos version supports. */
export function platformsQuery(version: string) {
  return staticQueryOptions({
    queryKey: ['platforms', version],
    queryFn: () => api.GET('/api/version/{version}/platforms', { params: { path: { version } } }),
  })
}

/** The single-board computers that a Talos version supports. */
export function boardsQuery(version: string) {
  return staticQueryOptions({
    queryKey: ['boards', version],
    queryFn: () => api.GET('/api/version/{version}/boards', { params: { path: { version } } }),
  })
}

/** The features of a Talos version. */
export function featuresQuery(version: string) {
  return staticQueryOptions({
    queryKey: ['features', version],
    queryFn: () => api.GET('/api/version/{version}/features', { params: { path: { version } } }),
  })
}

// The internal metal-agent extension isn't for users to pick, so the UI hides it.
const hiddenExtensions = ['siderolabs/metal-agent']

/** The official extensions of a Talos version that users can pick. */
export function extensionsQuery(version: string) {
  return staticQueryOptions({
    queryKey: ['extensions', version],
    queryFn: async () => {
      const extensions = await api.GET('/version/{version}/extensions/official', {
        params: { path: { version } },
      })

      return extensions.filter((e) => !hiddenExtensions.includes(e.name))
    },
  })
}

/** The canonical YAML of a schematic. A schematic ID names its content, so it never changes. */
export function schematicQuery(id: string) {
  return staticQueryOptions({
    queryKey: ['schematic', id],
    queryFn: () =>
      api.GET('/schematics/{schematic}', { params: { path: { schematic: id } }, parseAs: 'text' }),
  })
}

/** What to download a schematic's assets for. */
export interface AssetsFor {
  schematic: string
  version: string
  target: Target
  arch: Arch
  platform?: string
  board?: string
  secureBoot: boolean
}

/** The boot assets and installer images of a schematic. */
export function assetsQuery({ schematic, version, ...query }: AssetsFor) {
  return staticQueryOptions({
    queryKey: ['assets', schematic, version, query],
    queryFn: () =>
      api.GET('/api/schematics/{schematic}/{version}/assets', {
        params: {
          path: { schematic, version },
          query: {
            target: query.target,
            platform: query.platform,
            board: query.board,
            arch: query.arch,
            secureboot: query.secureBoot,
          },
        },
      }),
  })
}

/** The vulnerability scan report at a report URL, which the assets list. It is served outside the API. */
export function scanReportQuery(url: string) {
  return staticQueryOptions({
    queryKey: ['scan-report', url],
    queryFn: async () => {
      const response = await fetch(url, {
        headers: { Accept: 'application/json' },
        credentials: 'same-origin',
      })

      if (!response.ok) throw new Error(`HTTP ${response.status}`)

      return (await response.json()) as ScanReport
    },
  })
}

/** The actors that API tokens can be issued to. */
export function tokenActorsQuery() {
  return staticQueryOptions({
    queryKey: ['token-actors'],
    queryFn: () => api.GET('/api/tokens/actors'),
  })
}

/** The API tokens of the signed-in identity. Creating or revoking one loads them again. */
export function tokensQuery() {
  // Not static: tokens are also created and revoked elsewhere, such as with the API or another tab, and they
  // expire, so the list loads fresh each time the page opens.
  return queryOptions({
    queryKey: ['tokens'],
    queryFn: () => api.GET('/tokens'),
  })
}

/** Creates a schematic, and keeps its YAML so the final step doesn't fetch it again. */
export function createSchematicMutation() {
  return mutationOptions({
    mutationKey: ['create-schematic'],
    mutationFn: (schematic: Schemas['Schematic']) => api.POST('/schematics', { body: schematic }),
    onSuccess: (created, _schematic, _result, { client }) => {
      client.setQueryData(schematicQuery(created.id).queryKey, created.schematic)
    },
  })
}

/** Creates an API token. */
export function createTokenMutation() {
  return mutationOptions({
    mutationKey: ['create-token'],
    mutationFn: (request: Schemas['APITokenCreateRequest']) =>
      api.POST('/tokens', { body: request }),
    onSuccess: (_token, _request, _result, { client }) =>
      client.invalidateQueries({ queryKey: tokensQuery().queryKey }),
  })
}

/** Revokes an API token. */
export function revokeTokenMutation() {
  return mutationOptions({
    mutationKey: ['revoke-token'],
    mutationFn: (id: string) => api.POST('/tokens/{id}/revoke', { params: { path: { id } } }),
    onSuccess: (_data, _id, _result, { client }) =>
      client.invalidateQueries({ queryKey: tokensQuery().queryKey }),
  })
}
