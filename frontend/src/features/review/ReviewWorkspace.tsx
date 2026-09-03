import type {FindingDTO, FindingReview, SARIFDocumentDTO} from '../../../bindings/sastafras/backend/models'
import {DocumentSummary} from './DocumentSummary'
import {FindingBrowser} from './FindingBrowser'
import {FindingDetail} from './FindingDetail'
import type {Draft, EffectiveFinding, ReviewSummary, Severity} from './model'

export interface ReviewWorkspaceProps {
  document: SARIFDocumentDTO
  reviews: Map<string, FindingReview>
  selectedID: string
  selectedFinding: FindingDTO | null
  draft: Draft
  draftDirty: boolean
  validationError: string
  search: string
  severityFilter: string
  dispositionFilter: string
  runFilter: string
  page: number
  pageCount: number
  filteredFindings: EffectiveFinding[]
  pageFindings: EffectiveFinding[]
  summary: ReviewSummary
  reviewedCount: number
  onSearchChange: (value: string) => void
  onSeverityFilterChange: (value: string) => void
  onDispositionFilterChange: (value: string) => void
  onRunFilterChange: (value: string) => void
  onPageChange: (page: number) => void
  onSelectFinding: (finding: FindingDTO) => void
  onDraftChange: (draft: Draft) => void
  onValidationErrorChange: (error: string) => void
  onClearFilters: () => void
  onResetDraft: () => void
  onApplyReview: () => void
  onOpenHelp: (uri: string) => void
}

export function ReviewWorkspace(props: ReviewWorkspaceProps) {
  const toggleSeverity = (severity: Severity) => props.onSeverityFilterChange(props.severityFilter === severity ? 'all' : severity)

  return <main className="workspace">
    <DocumentSummary document={props.document} summary={props.summary} reviewedCount={props.reviewedCount} severityFilter={props.severityFilter} onToggleSeverity={toggleSeverity}/>
    <section className="review-layout">
      <FindingBrowser
        document={props.document}
        filteredFindings={props.filteredFindings}
        pageFindings={props.pageFindings}
        selectedID={props.selectedID}
        search={props.search}
        severityFilter={props.severityFilter}
        dispositionFilter={props.dispositionFilter}
        runFilter={props.runFilter}
        page={props.page}
        pageCount={props.pageCount}
        onSearchChange={props.onSearchChange}
        onSeverityFilterChange={props.onSeverityFilterChange}
        onDispositionFilterChange={props.onDispositionFilterChange}
        onRunFilterChange={props.onRunFilterChange}
        onSelectFinding={props.onSelectFinding}
        onClearFilters={props.onClearFilters}
        onPageChange={props.onPageChange}
      />
      <article className="finding-detail">
        {props.selectedFinding ? <FindingDetail
          finding={props.selectedFinding}
          draft={props.draft}
          draftDirty={props.draftDirty}
          validationError={props.validationError}
          appliedReview={props.reviews.get(props.selectedID)}
          onDraftChange={props.onDraftChange}
          onValidationErrorChange={props.onValidationErrorChange}
          onOpenHelp={props.onOpenHelp}
          onReset={props.onResetDraft}
          onApply={props.onApplyReview}
        /> : <div className="detail-empty">Select a finding to review its details.</div>}
      </article>
    </section>
  </main>
}
