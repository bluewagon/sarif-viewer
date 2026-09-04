import type {FindingDTO, SARIFDocumentDTO} from '../../../bindings/sarif-viewer/backend/models'
import {icons} from '../../components/icons'
import {
  dispositions,
  findingID,
  locationLabel,
  PAGE_SIZE,
  severities,
  titleCase,
  type Draft,
  type EffectiveFinding,
} from './model'

interface FindingRowProps {
  finding: FindingDTO
  review: Draft
  selected: boolean
  onSelect: (finding: FindingDTO) => void
}

function FindingRow({finding, review, selected, onSelect}: FindingRowProps) {
  return <button className={`finding-row ${selected ? 'is-selected' : ''}`} onClick={() => onSelect(finding)}><span className={`severity-bar severity-bg-${review.severity}`}/><span className="finding-content"><span className="finding-topline"><strong>{finding.ruleId || 'Unidentified rule'}</strong><span className={`severity-pill severity-${review.severity}`}>{titleCase(review.severity)}</span></span><span className="finding-message">{finding.message}</span><span className="finding-meta"><span>{icons.location}{locationLabel(finding)}</span><span className={`disposition disposition-${review.disposition}`}>{titleCase(review.disposition)}</span></span></span></button>
}

interface FindingBrowserProps {
  document: SARIFDocumentDTO
  filteredFindings: EffectiveFinding[]
  pageFindings: EffectiveFinding[]
  selectedID: string
  search: string
  severityFilter: string
  dispositionFilter: string
  runFilter: string
  page: number
  pageCount: number
  onSearchChange: (value: string) => void
  onSeverityFilterChange: (value: string) => void
  onDispositionFilterChange: (value: string) => void
  onRunFilterChange: (value: string) => void
  onSelectFinding: (finding: FindingDTO) => void
  onClearFilters: () => void
  onPageChange: (page: number) => void
}

export function FindingBrowser({
  document,
  filteredFindings,
  pageFindings,
  selectedID,
  search,
  severityFilter,
  dispositionFilter,
  runFilter,
  page,
  pageCount,
  onSearchChange,
  onSeverityFilterChange,
  onDispositionFilterChange,
  onRunFilterChange,
  onSelectFinding,
  onClearFilters,
  onPageChange,
}: FindingBrowserProps) {
  return <aside className="finding-browser" aria-label="Findings">
    <div className="browser-toolbar">
      <label className="search-box">{icons.search}<span className="sr-only">Search findings</span><input value={search} onChange={(event) => onSearchChange(event.target.value)} placeholder="Search findings…" /></label>
      <div className="filter-row">
        <label><span className="sr-only">Run</span><select value={runFilter} onChange={(event) => onRunFilterChange(event.target.value)}><option value="all">All runs</option>{document.runs?.map((run) => <option key={run.index} value={run.index}>{run.name}</option>)}</select></label>
        <label><span className="sr-only">Severity</span><select value={severityFilter} onChange={(event) => onSeverityFilterChange(event.target.value)}><option value="all">All severities</option>{severities.map((severity) => <option key={severity} value={severity}>{titleCase(severity)}</option>)}</select></label>
        <label><span className="sr-only">Disposition</span><select value={dispositionFilter} onChange={(event) => onDispositionFilterChange(event.target.value)}><option value="all">All statuses</option>{dispositions.map((disposition) => <option key={disposition} value={disposition}>{titleCase(disposition)}</option>)}</select></label>
      </div>
    </div>
    <div className="result-count"><strong>{filteredFindings.length.toLocaleString()}</strong> {filteredFindings.length === 1 ? 'finding' : 'findings'}</div>
    <div className="finding-list">
      {pageFindings.length ? pageFindings.map(({finding, review}) => <FindingRow key={findingID(finding)} finding={finding} review={review} selected={findingID(finding) === selectedID} onSelect={onSelectFinding}/>) : <div className="no-results"><strong>No matching findings</strong><span>Try changing your search or filters.</span><button onClick={onClearFilters}>Clear filters</button></div>}
    </div>
    {filteredFindings.length > PAGE_SIZE && <nav className="pagination" aria-label="Finding pages"><button onClick={() => onPageChange(Math.max(0, page - 1))} disabled={page === 0} aria-label="Previous page">‹</button><span>Page {page + 1} of {pageCount}</span><button onClick={() => onPageChange(Math.min(pageCount - 1, page + 1))} disabled={page >= pageCount - 1} aria-label="Next page">›</button></nav>}
  </aside>
}
