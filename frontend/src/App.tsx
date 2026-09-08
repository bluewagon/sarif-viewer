import {AppHeader} from './components/AppHeader'
import {FeedbackBanner} from './components/FeedbackBanner'
import {ReviewerDialog} from './components/ReviewerDialog'
import {Welcome} from './components/Welcome'
import {SourceImportModal} from './features/import/SourceImportModal'
import {ReviewWorkspace} from './features/review/ReviewWorkspace'
import {useSarifReviewController} from './hooks/useSarifReviewController'

function App() {
  const controller = useSarifReviewController()

  return <div className="app-shell">
    <AppHeader
      document={controller.document}
      dirty={controller.dirty}
      loading={controller.loading}
      onOpen={controller.openSARIF}
      onExport={controller.exportSARIF}
      reviewer={controller.reviewer}
      onEditReviewer={controller.editReviewer}
    />
    {controller.feedback && <FeedbackBanner feedback={controller.feedback} onDismiss={controller.dismissFeedback}/>}
    {controller.sourcePrompt && <SourceImportModal
      prompt={controller.sourcePrompt}
      loading={controller.loading}
      onChange={controller.setSourcePrompt}
      onChooseFolder={controller.chooseSourceFolder}
      onCancel={() => controller.setSourcePrompt(null)}
      onLoad={controller.loadWithSource}
    />}
    {!controller.document ? <Welcome loading={controller.loading} onOpen={controller.openSARIF}/> : <ReviewWorkspace
      document={controller.document}
      reviews={controller.reviews}
      selectedID={controller.selectedID}
      selectedFinding={controller.selectedFinding}
      draft={controller.draft}
      draftDirty={controller.draftDirty}
      validationError={controller.validationError}
      search={controller.search}
      severityFilter={controller.severityFilter}
      dispositionFilter={controller.dispositionFilter}
      runFilter={controller.runFilter}
      page={controller.page}
      pageCount={controller.pageCount}
      filteredFindings={controller.filteredFindings}
      pageFindings={controller.pageFindings}
      summary={controller.summary}
      reviewedCount={controller.reviewedCount}
      onSearchChange={controller.setSearch}
      onSeverityFilterChange={controller.setSeverityFilter}
      onDispositionFilterChange={controller.setDispositionFilter}
      onRunFilterChange={controller.setRunFilter}
      onPageChange={controller.setPage}
      onSelectFinding={controller.selectFinding}
      onDraftChange={controller.setDraft}
      onValidationErrorChange={controller.setValidationError}
      onClearFilters={controller.clearFilters}
      onResetDraft={controller.resetDraft}
      onApplyReview={controller.applyReview}
      onOpenHelp={controller.openHelp}
    />}
    {controller.reviewerDialogOpen && <ReviewerDialog
      value={controller.reviewerDraft}
      error={controller.reviewerError}
      canCancel={Boolean(controller.reviewer)}
      onChange={controller.setReviewerDraft}
      onConfirm={controller.confirmReviewer}
      onCancel={controller.cancelReviewerEdit}
    />}
  </div>
}

export default App
