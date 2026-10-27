// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import createClient, { type Middleware } from 'openapi-fetch'

import type { components, paths } from '@/api/schema'

/**
 * A 401 means the session ended. Reloading lets the backend take over: it redirects to the login page, or asks
 * the browser for credentials.
 */
const reloadOnUnauthorized: Middleware = {
  onResponse({ response }) {
    if (response.status === 401) {
      window.location.reload()
    }
  },
}

const client = createClient<paths>({
  // Resolved on every request, so that tests can stub the global fetch.
  fetch: (request) => globalThis.fetch(request),
})

client.use(reloadOnUnauthorized)

/** The data of a successful response. */
type Data<R> = R extends { data: infer D; error?: never } ? D : never

/** Wraps a client method to return the data of the response, and throw an APIError when the request failed. */
function returningData<
  A extends unknown[],
  R extends { data?: unknown; error?: unknown; response: Response },
>(method: (...args: A) => Promise<R>) {
  return async (...args: A): Promise<Data<R>> => {
    const { data, error, response } = await method(...args)

    if (!response.ok) {
      throw new APIError(errorMessage(error, response))
    }

    return data as Data<R>
  }
}

/**
 * Typed client for the Image Factory API, generated from api/openapi.yaml. Its methods return the response data,
 * and throw an APIError with the server's message when the request fails.
 * Run `npm run generate` after changing the OpenAPI contract.
 */
export const api = {
  GET: returningData(client.GET),
  POST: returningData(client.POST),
}

export type Schemas = components['schemas']

/** APIError carries the message of a failed API request. */
export class APIError extends Error {}

// maxErrorLength bounds what a failed response is allowed to put on the page. The server's messages are one line;
// anything longer, or markup, came from a proxy in front of it.
const maxErrorLength = 300

function errorMessage(error: unknown, response: Response) {
  const message = (typeof error === 'string' ? error : JSON.stringify(error ?? '')).trim()

  if (!message || message === '""' || message.length > maxErrorLength || message.startsWith('<')) {
    return `HTTP ${response.status}`
  }

  return message
}
