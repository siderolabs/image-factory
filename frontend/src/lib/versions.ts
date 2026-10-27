// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

import major from 'semver/functions/major'
import minor from 'semver/functions/minor'
import parse from 'semver/functions/parse'
import prerelease from 'semver/functions/prerelease'
import rcompare from 'semver/functions/rcompare'
import valid from 'semver/functions/valid'

/** A group of versions in the version picker: one minor release, or its pre-releases. */
export interface VersionGroup {
  label: string
  versions: string[]
}

/** The versions to pick from, grouped by minor release, newest first. */
export interface VersionChoices {
  groups: VersionGroup[]
  /** The newest version that isn't a pre-release, which is the default choice. */
  latestStable?: string
}

/** Groups Talos versions (such as v1.13.0) for the version picker; the versions lose their leading v. */
export function groupVersions(versions: string[]): VersionChoices {
  const sorted = versions
    .map((v) => v.replace(/^v/, ''))
    .filter((v) => valid(v))
    .sort(rcompare)

  // Sorted newest first, each group starts at its newest version, so the groups come out newest first too, with
  // a minor release before its pre-releases.
  const groups = sorted.reduce<VersionGroup[]>((groups, version) => {
    const label = `${major(version)}.${minor(version)}${prerelease(version) ? '-pre' : ''}`
    const group = groups.find((g) => g.label === label)

    if (group) {
      group.versions.push(version)
    } else {
      groups.push({ label, versions: [version] })
    }

    return groups
  }, [])

  return { groups, latestStable: sorted.find((v) => !prerelease(v)) }
}

/** The major and minor version, such as v1.13, as the Talos documentation is versioned. */
export function shortVersion(version: string): string {
  const v = parse(version)

  return v ? `v${v.major}.${v.minor}` : version
}
