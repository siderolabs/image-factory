// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { delay, http, HttpResponse } from 'msw'

import type { paths } from '@/api/schema'

/** The MSW path of an API path, such as /version/:version for /version/{version}. */
export function apiPath(path: keyof paths) {
  return path.replace(/\{([^}]+)\}/g, ':$1')
}

/** Answers an API request with an error, the way the server does: its message as plain text. */
export function apiError(
  method: 'get' | 'post',
  path: keyof paths,
  status: number,
  message = 'internal server error',
) {
  return http[method](apiPath(path), () => HttpResponse.text(message, { status }))
}

/** Never answers an API request, to show what waits on it loading. */
export function apiPending(method: 'get' | 'post', path: keyof paths) {
  return http[method](apiPath(path), () => delay('infinite'))
}
