// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import { shortVersion } from '@/lib/versions'

/** A page of the Talos documentation for a version; path starts with a slash. */
export function docsURL(version: string, path: string): string {
  return `https://docs.siderolabs.com/talos/${shortVersion(version)}${path}`
}

/** The documentation page on Enterprise Image Factory, linked wherever a feature needs it. */
export const enterpriseDocsURL =
  'https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory'
