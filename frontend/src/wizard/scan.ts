// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

/** A vulnerability, as grype's JSON report describes it; only the fields the UI shows. */
export interface Vulnerability {
  id?: string
  severity?: string
  description?: string
  dataSource?: string
  urls?: string[]
  fix?: { versions?: string[] }
}

/** A vulnerability that grype matched against a package of the image. */
export interface Match {
  vulnerability?: Vulnerability
  relatedVulnerabilities?: Vulnerability[]
  artifact?: { name?: string; version?: string }
  matchDetails?: {
    matcher?: string
    found?: {
      Statement?: {
        status?: string
        justification?: string
        impact_statement?: string
        action_statement?: string
      }
    }
  }[]
}

/** grype's JSON report. */
export interface ScanReport {
  matches?: Match[]
}

/** The severities, most severe first. */
export const severities = ['Critical', 'High', 'Medium', 'Low', 'Negligible', 'Unknown'] as const

export type Severity = (typeof severities)[number]

function isCVE(v?: Vulnerability) {
  return !!v?.id?.startsWith('CVE-')
}

/** The CVE ID of a match, which may be one of its related vulnerabilities, or its own ID otherwise. */
export function preferredCVE(match: Match): string {
  const v = match.vulnerability

  if (isCVE(v)) return v?.id ?? ''

  return match.relatedVulnerabilities?.find(isCVE)?.id ?? v?.id ?? ''
}

function urls(v?: Vulnerability): string[] {
  return v ? [...(v.dataSource ? [v.dataSource] : []), ...(v.urls ?? [])] : []
}

/** The best page about a match: one on the NVD if there is one, preferring the CVE records. */
export function bestURL(match: Match): string {
  const v = match.vulnerability

  let pool = [
    ...(isCVE(v) ? urls(v) : []),
    ...(match.relatedVulnerabilities ?? []).filter(isCVE).flatMap(urls),
  ]

  if (!pool.length) pool = urls(v)

  return pool.find((u) => u.includes('nvd.nist.gov')) ?? pool[0] ?? ''
}

/** The versions that fix a match. */
export function fixVersions(match: Match): string {
  return match.vulnerability?.fix?.versions?.join(', ') ?? ''
}

/** The OpenVEX statements that grype attached to a match, deduplicated by their text. */
export function vexStatements(match: Match): string[] {
  const out = new Set<string>()

  for (const detail of match.matchDetails ?? []) {
    if (detail.matcher !== 'openvex-matcher') continue

    const statement = detail.found?.Statement

    if (!statement) continue

    const parts = [
      statement.status && `Status: ${statement.status.replace(/_/g, ' ')}`,
      statement.justification && `Justification: ${statement.justification.replace(/_/g, ' ')}`,
      statement.impact_statement,
      statement.action_statement,
    ].filter(Boolean)

    if (parts.length) out.add(parts.join('\n'))
  }

  return [...out]
}
