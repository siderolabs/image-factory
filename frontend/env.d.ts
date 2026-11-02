// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

/// <reference types="vite/client" />

/** Design token colours per theme. Set by vite-theme-init.ts. */
declare const __THEME_COLORS__: Record<
  import('@/lib/theme-core').Theme,
  { page: string; chrome: string }
>
