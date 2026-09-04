import type {SARIFDocumentDTO} from '../../../bindings/sarif-viewer/backend/models'
import {icons} from '../../components/icons'
import {severities, titleCase, type ReviewSummary, type Severity} from './model'

interface DocumentSummaryProps {
  document: SARIFDocumentDTO
  summary: ReviewSummary
  reviewedCount: number
  severityFilter: string
  onToggleSeverity: (severity: Severity) => void
}

export function DocumentSummary({document, summary, reviewedCount, severityFilter, onToggleSeverity}: DocumentSummaryProps) {
  return <>
    <section className="document-heading">
      <div><div className="document-title-row"><span className="document-file-icon">{icons.file}</span><h1>{document.fileName}</h1><span className="version-badge">SARIF {document.version}</span></div><p title={document.sourcePath}>{document.runs?.map((run) => run.toolName).filter((name, index, all) => all.indexOf(name) === index).join(' · ')} · {document.runs?.length ?? 0} {(document.runs?.length ?? 0) === 1 ? 'run' : 'runs'}</p></div>
      <div className="headline-count"><strong>{document.findingCount.toLocaleString()}</strong><span>Total findings</span></div>
    </section>
    {document.snippetSummary && <div className="snippet-summary" role="status"><strong>{document.snippetSummary.generated.toLocaleString()}</strong> generated · <strong>{document.snippetSummary.embedded.toLocaleString()}</strong> embedded · <strong>{document.snippetSummary.unavailable.toLocaleString()}</strong> unavailable</div>}
    <section className="summary-grid" aria-label="Finding summary">
      {severities.map((severity) => <button key={severity} className={`summary-card severity-${severity} ${severityFilter === severity ? 'is-active' : ''}`} onClick={() => onToggleSeverity(severity)}><span>{titleCase(severity)}</span><strong>{summary.severityCounts[severity].toLocaleString()}</strong></button>)}
      <div className="review-progress"><div><span>Reviewed</span><strong>{reviewedCount.toLocaleString()} <small>/ {document.findingCount.toLocaleString()}</small></strong></div><div className="progress-track"><span style={{width: `${document.findingCount ? (reviewedCount / document.findingCount) * 100 : 0}%`}} /></div></div>
    </section>
  </>
}
