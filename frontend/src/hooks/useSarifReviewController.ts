import {useEffect, useMemo, useState} from 'react'
import {Browser, Dialogs} from '@wailsio/runtime'
import {SARIFService} from '../../bindings/sarif-viewer/backend'
import type {FindingDTO, FindingReview, SARIFDocumentDTO, SourceSelectionDTO} from '../../bindings/sarif-viewer/backend/models'
import {createSourcePrompt, validateSourceSelection, type SourcePrompt} from '../features/import/model'
import {
  effectiveReview,
  findingID,
  PAGE_SIZE,
  reviewedFilename,
  summarizeFindings,
  type Draft,
  type EffectiveFinding,
  type Feedback,
  type ReviewSummary,
} from '../features/review/model'

export const REVIEWER_STORAGE_KEY = 'sarif-viewer.reviewerUsername'

function savedReviewer(): string {
  try {
    return window.localStorage.getItem(REVIEWER_STORAGE_KEY) ?? ''
  } catch {
    return ''
  }
}

export interface SarifReviewController {
  document: SARIFDocumentDTO | null
  reviews: Map<string, FindingReview>
  selectedID: string
  selectedFinding: FindingDTO | null
  draft: Draft
  draftDirty: boolean
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
  loading: boolean
  validationError: string
  feedback: Feedback
  dirty: boolean
  reviewer: string
  reviewerDraft: string
  reviewerDialogOpen: boolean
  reviewerError: string
  sourcePrompt: SourcePrompt | null
  openSARIF: () => Promise<void>
  chooseSourceFolder: () => Promise<void>
  loadWithSource: (source: SourceSelectionDTO) => Promise<void>
  exportSARIF: () => Promise<void>
  selectFinding: (finding: FindingDTO) => void
  applyReview: () => void
  resetDraft: () => void
  clearFilters: () => void
  openHelp: (uri: string) => void
  setDraft: (draft: Draft) => void
  setSearch: (value: string) => void
  setSeverityFilter: (value: string) => void
  setDispositionFilter: (value: string) => void
  setRunFilter: (value: string) => void
  setPage: (page: number) => void
  setValidationError: (error: string) => void
  setSourcePrompt: (prompt: SourcePrompt | null) => void
  setReviewerDraft: (reviewer: string) => void
  confirmReviewer: () => void
  editReviewer: () => void
  cancelReviewerEdit: () => void
  dismissFeedback: () => void
}

export function useSarifReviewController(): SarifReviewController {
  const [document, setDocument] = useState<SARIFDocumentDTO | null>(null)
  const [reviews, setReviews] = useState<Map<string, FindingReview>>(new Map())
  const [selectedID, setSelectedID] = useState('')
  const [draft, setDraft] = useState<Draft>({severity: 'medium', disposition: 'unreviewed', comment: ''})
  const [search, setSearch] = useState('')
  const [severityFilter, setSeverityFilter] = useState('all')
  const [dispositionFilter, setDispositionFilter] = useState('all')
  const [runFilter, setRunFilter] = useState('all')
  const [page, setPage] = useState(0)
  const [loading, setLoading] = useState(false)
  const [validationError, setValidationError] = useState('')
  const [feedback, setFeedback] = useState<Feedback>(null)
  const [dirty, setDirty] = useState(false)
  const [reviewer, setReviewer] = useState('')
  const [reviewerDraft, setReviewerDraftValue] = useState(savedReviewer)
  const [reviewerDialogOpen, setReviewerDialogOpen] = useState(true)
  const [reviewerError, setReviewerError] = useState('')
  const [sourcePrompt, setSourcePrompt] = useState<SourcePrompt | null>(null)

  const findings = useMemo(() => document?.findings ?? [], [document])
  const selectedFinding = useMemo(() => findings.find((finding) => findingID(finding) === selectedID) ?? null, [findings, selectedID])
  const currentReview = selectedFinding ? effectiveReview(selectedFinding, reviews) : null
  const draftDirty = currentReview !== null && (draft.severity !== currentReview.severity || draft.disposition !== currentReview.disposition || draft.comment !== currentReview.comment)
  const effectiveFindings = useMemo(() => findings.map((finding) => ({finding, review: effectiveReview(finding, reviews)})), [findings, reviews])

  const filteredFindings = useMemo(() => {
    const query = search.trim().toLowerCase()
    return effectiveFindings.filter(({finding, review}) => {
      if (severityFilter !== 'all' && review.severity !== severityFilter) return false
      if (dispositionFilter !== 'all' && review.disposition !== dispositionFilter) return false
      if (runFilter !== 'all' && finding.key.runIndex !== Number(runFilter)) return false
      if (!query) return true
      return [finding.ruleId, finding.ruleName, finding.message, finding.location.uri, finding.runName, finding.toolName].some((value) => value.toLowerCase().includes(query))
    })
  }, [effectiveFindings, search, severityFilter, dispositionFilter, runFilter])

  const pageCount = Math.max(1, Math.ceil(filteredFindings.length / PAGE_SIZE))
  const pageFindings = filteredFindings.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE)
  const summary = useMemo(() => summarizeFindings(effectiveFindings), [effectiveFindings])
  const reviewedCount = summary.dispositionCounts.confirmed + summary.dispositionCounts['false-positive']

  useEffect(() => setPage(0), [search, severityFilter, dispositionFilter, runFilter])
  useEffect(() => {
    if (!selectedFinding) return
    setDraft(effectiveReview(selectedFinding, reviews))
    setValidationError('')
  }, [selectedID, selectedFinding, reviews])
  useEffect(() => {
    if (!feedback) return
    const timer = window.setTimeout(() => setFeedback(null), 5000)
    return () => window.clearTimeout(timer)
  }, [feedback])

  async function confirmDiscard(message: string): Promise<boolean> {
    const choice = await Dialogs.Question({
      Title: 'Unsaved review changes',
      Message: message,
      Buttons: [{Label: 'Keep reviewing', IsCancel: true}, {Label: 'Discard changes', IsDefault: true}],
    })
    return choice === 'Discard changes'
  }

  async function openSARIF() {
    if ((dirty || draftDirty) && !await confirmDiscard('Opening another file will discard review changes that have not been exported.')) return
    const path = await Dialogs.OpenFile({
      Title: 'Open SARIF file',
      ButtonText: 'Open SARIF',
      CanChooseFiles: true,
      AllowsMultipleSelection: false,
      AllowsOtherFiletypes: true,
      Filters: [{DisplayName: 'SARIF files', Pattern: '*.sarif;*.json'}],
    })
    if (!path) return
    setFeedback(null)
    setSourcePrompt(createSourcePrompt(path))
  }

  async function chooseSourceFolder() {
    if (!sourcePrompt || loading) return
    const path = await Dialogs.OpenFile({
      Title: 'Choose source folder',
      ButtonText: 'Choose Folder',
      CanChooseDirectories: true,
      CanChooseFiles: false,
      AllowsMultipleSelection: false,
      AllowsOtherFiletypes: true,
    })
    if (path) setSourcePrompt({...sourcePrompt, kind: 'local', location: path, error: ''})
  }

  async function loadWithSource(source: SourceSelectionDTO) {
    if (!sourcePrompt) return
    const error = validateSourceSelection(source)
    if (error) {
      setSourcePrompt({...sourcePrompt, error})
      return
    }

    const sarifPath = sourcePrompt.sarifPath
    setLoading(true)
    setSourcePrompt({...sourcePrompt, gitPersonalAccessToken: '', revealGitPersonalAccessToken: false, error: ''})
    try {
      const loaded = await SARIFService.LoadSARIF(sarifPath, source)
      setDocument(loaded)
      setReviews(new Map())
      setDirty(false)
      setSearch('')
      setSeverityFilter('all')
      setDispositionFilter('all')
      setRunFilter('all')
      setPage(0)
      setSelectedID(loaded.findings?.[0] ? findingID(loaded.findings[0]) : '')
      setSourcePrompt(null)
    } catch (caughtError) {
      const message = caughtError instanceof Error ? caughtError.message : String(caughtError)
      setSourcePrompt((current) => current ? {...current, error: message} : current)
    } finally {
      setLoading(false)
    }
  }

  function selectFinding(finding: FindingDTO) {
    const nextID = findingID(finding)
    if (nextID === selectedID) return
    if (draftDirty && !window.confirm('Discard the unapplied changes to this finding?')) return
    setSelectedID(nextID)
  }

  function applyReview() {
    if (!selectedFinding || !draftDirty) return
    if (!reviewer) {
      setReviewerDraftValue(savedReviewer())
      setReviewerError('Enter a username before applying a review.')
      setReviewerDialogOpen(true)
      return
    }
    const comment = draft.comment.trim()
    if (!comment) {
      setValidationError('Add a comment before applying this review.')
      return
    }
    const review: FindingReview = {
      runIndex: selectedFinding.key.runIndex,
      resultIndex: selectedFinding.key.resultIndex,
      severity: draft.severity,
      disposition: draft.disposition,
      comment,
      reviewer,
      reviewedAt: new Date().toISOString(),
    }
    setReviews((current) => new Map(current).set(findingID(review), review))
    setDraft((current) => ({...current, comment}))
    setValidationError('')
    setDirty(true)
    setFeedback({kind: 'success', message: `Review applied to ${selectedFinding.ruleId || 'finding'}.`})
  }

  function resetDraft() {
    if (!currentReview) return
    setDraft(currentReview)
    setValidationError('')
  }

  async function exportSARIF() {
    if (!document || !dirty) return
    const destination = await Dialogs.SaveFile({
      Title: 'Export reviewed SARIF',
      ButtonText: 'Export',
      Filename: reviewedFilename(document.fileName),
      CanCreateDirectories: true,
      AllowsOtherFiletypes: false,
      Filters: [{DisplayName: 'SARIF files', Pattern: '*.sarif'}],
    })
    if (!destination) return
    setFeedback(null)
    try {
      await SARIFService.ExportSARIF(document.documentId, destination, Array.from(reviews.values()))
      setDirty(false)
      setFeedback({kind: 'success', message: 'Reviewed SARIF exported successfully.'})
    } catch (error) {
      setFeedback({kind: 'error', message: error instanceof Error ? error.message : String(error)})
    }
  }

  function clearFilters() {
    setSearch('')
    setSeverityFilter('all')
    setDispositionFilter('all')
    setRunFilter('all')
  }

  function setReviewerDraft(value: string) {
    setReviewerDraftValue(value)
    setReviewerError('')
  }

  function confirmReviewer() {
    const value = reviewerDraft.trim()
    if (!value) {
      setReviewerError('Enter your username to continue.')
      return
    }
    setReviewer(value)
    setReviewerDraftValue(value)
    setReviewerError('')
    setReviewerDialogOpen(false)
    try {
      window.localStorage.setItem(REVIEWER_STORAGE_KEY, value)
    } catch {
      // Local persistence is optional; the active reviewer remains available in memory.
    }
  }

  function editReviewer() {
    setReviewerDraftValue(reviewer)
    setReviewerError('')
    setReviewerDialogOpen(true)
  }

  function cancelReviewerEdit() {
    if (!reviewer) return
    setReviewerDraftValue(reviewer)
    setReviewerError('')
    setReviewerDialogOpen(false)
  }

  return {
    document,
    reviews,
    selectedID,
    selectedFinding,
    draft,
    draftDirty,
    search,
    severityFilter,
    dispositionFilter,
    runFilter,
    page,
    pageCount,
    filteredFindings,
    pageFindings,
    summary,
    reviewedCount,
    loading,
    validationError,
    feedback,
    dirty,
    reviewer,
    reviewerDraft,
    reviewerDialogOpen,
    reviewerError,
    sourcePrompt,
    openSARIF,
    chooseSourceFolder,
    loadWithSource,
    exportSARIF,
    selectFinding,
    applyReview,
    resetDraft,
    clearFilters,
    openHelp: (uri) => Browser.OpenURL(uri),
    setDraft,
    setSearch,
    setSeverityFilter,
    setDispositionFilter,
    setRunFilter,
    setPage,
    setValidationError,
    setSourcePrompt,
    setReviewerDraft,
    confirmReviewer,
    editReviewer,
    cancelReviewerEdit,
    dismissFeedback: () => setFeedback(null),
  }
}
