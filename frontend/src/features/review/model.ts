import type {FindingDTO, FindingReview} from '../../../bindings/sastafras/backend/models'

export const PAGE_SIZE = 100
export const severities = ['critical', 'high', 'medium', 'low', 'informational'] as const
export const dispositions = ['unreviewed', 'confirmed', 'false-positive'] as const

export type Severity = typeof severities[number]
export type Disposition = typeof dispositions[number]
export type Draft = Pick<FindingReview, 'severity' | 'disposition' | 'comment'>
export type Feedback = {kind: 'success' | 'error'; message: string} | null
export type EffectiveFinding = {finding: FindingDTO; review: Draft}
export type ReviewSummary = {
  severityCounts: Record<Severity, number>
  dispositionCounts: Record<Disposition, number>
}

export function findingID(finding: FindingDTO | FindingReview): string {
  return 'key' in finding ? `${finding.key.runIndex}:${finding.key.resultIndex}` : `${finding.runIndex}:${finding.resultIndex}`
}

export function titleCase(value: string): string {
  return value.split('-').map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ')
}

export function locationLabel(finding: FindingDTO): string {
  const {uri, startLine, startColumn} = finding.location
  if (!uri) return 'No location provided'
  if (!startLine) return uri
  return `${uri}:${startLine}${startColumn ? `:${startColumn}` : ''}`
}

export function reviewedFilename(fileName: string): string {
  return `${fileName.replace(/\.(sarif|json)$/i, '')}.reviewed.sarif`
}

export function effectiveReview(finding: FindingDTO, reviews: Map<string, FindingReview>): Draft {
  const review = reviews.get(findingID(finding))
  return {
    severity: review?.severity ?? finding.severity,
    disposition: review?.disposition ?? finding.disposition,
    comment: review?.comment ?? finding.comment,
  }
}

export function summarizeFindings(findings: EffectiveFinding[]): ReviewSummary {
  const severityCounts = Object.fromEntries(severities.map((severity) => [severity, 0])) as Record<Severity, number>
  const dispositionCounts = Object.fromEntries(dispositions.map((disposition) => [disposition, 0])) as Record<Disposition, number>
  findings.forEach(({review}) => {
    if (review.severity in severityCounts) severityCounts[review.severity as Severity] += 1
    if (review.disposition in dispositionCounts) dispositionCounts[review.disposition as Disposition] += 1
  })
  return {severityCounts, dispositionCounts}
}
