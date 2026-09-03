import {useEffect, useMemo, useState} from 'react'
import {Browser, Dialogs} from '@wailsio/runtime'
import {SARIFService} from '../bindings/sastafras/backend'
import type {FindingDTO, FindingReview, GitAuthenticationDTO, SARIFDocumentDTO, SourceSelectionDTO} from '../bindings/sastafras/backend/models'

const PAGE_SIZE = 100
const severities = ['critical', 'high', 'medium', 'low', 'informational'] as const
const dispositions = ['unreviewed', 'confirmed', 'false-positive'] as const
type Severity = typeof severities[number]
type Disposition = typeof dispositions[number]
type Draft = Pick<FindingReview, 'severity' | 'disposition' | 'comment'>
type Feedback = {kind: 'success' | 'error'; message: string} | null
type SourceKind = 'local' | 'git'
type GitProvider = 'existing' | 'github' | 'bitbucket' | 'azure-devops'
type SourcePrompt = {
  sarifPath: string
  kind: SourceKind
  location: string
  contextLines: number
  gitProvider: GitProvider
  gitUsername: string
  gitPersonalAccessToken: string
  revealGitPersonalAccessToken: boolean
  error: string
}

const emptyGitAuthentication: GitAuthenticationDTO = {provider: '', username: '', personalAccessToken: ''}

const icons = {
  folder: <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 6.5h6l2 2h10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-11Z"/><path d="M3 9h18"/></svg>,
  export: <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M5 20h14"/></svg>,
  search: <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m16 16 5 5"/></svg>,
  file: <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 2h8l4 4v16H6z"/><path d="M14 2v5h5"/></svg>,
  location: <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 10c0 5-8 12-8 12S4 15 4 10a8 8 0 1 1 16 0Z"/><circle cx="12" cy="10" r="2.5"/></svg>,
  external: <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M14 4h6v6"/><path d="m20 4-9 9"/><path d="M19 13v6a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1h6"/></svg>,
  check: <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m5 12 4 4L19 6"/></svg>,
}

function findingID(finding: FindingDTO | FindingReview): string {
  return 'key' in finding ? `${finding.key.runIndex}:${finding.key.resultIndex}` : `${finding.runIndex}:${finding.resultIndex}`
}

function titleCase(value: string): string {
  return value.split('-').map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ')
}

function locationLabel(finding: FindingDTO): string {
  const {uri, startLine, startColumn} = finding.location
  if (!uri) return 'No location provided'
  if (!startLine) return uri
  return `${uri}:${startLine}${startColumn ? `:${startColumn}` : ''}`
}

function reviewedFilename(fileName: string): string {
  return `${fileName.replace(/\.(sarif|json)$/i, '')}.reviewed.sarif`
}

function effectiveReview(finding: FindingDTO, reviews: Map<string, FindingReview>): Draft {
  const review = reviews.get(findingID(finding))
  return {
    severity: review?.severity ?? finding.severity,
    disposition: review?.disposition ?? finding.disposition,
    comment: review?.comment ?? finding.comment,
  }
}

function App() {
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
  const summary = useMemo(() => {
    const severityCounts = Object.fromEntries(severities.map((severity) => [severity, 0])) as Record<Severity, number>
    const dispositionCounts = Object.fromEntries(dispositions.map((disposition) => [disposition, 0])) as Record<Disposition, number>
    effectiveFindings.forEach(({review}) => {
      if (review.severity in severityCounts) severityCounts[review.severity as Severity] += 1
      if (review.disposition in dispositionCounts) dispositionCounts[review.disposition as Disposition] += 1
    })
    return {severityCounts, dispositionCounts}
  }, [effectiveFindings])

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
      Title: 'Unsaved review changes', Message: message,
      Buttons: [{Label: 'Keep reviewing', IsCancel: true}, {Label: 'Discard changes', IsDefault: true}],
    })
    return choice === 'Discard changes'
  }

  async function openSARIF() {
    if ((dirty || draftDirty) && !await confirmDiscard('Opening another file will discard review changes that have not been exported.')) return
    const path = await Dialogs.OpenFile({
      Title: 'Open SARIF file', ButtonText: 'Open SARIF', CanChooseFiles: true, AllowsMultipleSelection: false,
      AllowsOtherFiletypes: true, Filters: [{DisplayName: 'SARIF files', Pattern: '*.sarif;*.json'}],
    })
    if (!path) return
    setFeedback(null)
    setSourcePrompt({
      sarifPath: path,
      kind: 'local',
      location: '',
      contextLines: 3,
      gitProvider: 'github',
      gitUsername: '',
      gitPersonalAccessToken: '',
      revealGitPersonalAccessToken: false,
      error: '',
    })
  }

  async function chooseSourceFolder() {
    if (!sourcePrompt || loading) return
    const path = await Dialogs.OpenFile({
      Title: 'Choose source folder', ButtonText: 'Choose Folder', CanChooseDirectories: true, CanChooseFiles: false,
      AllowsMultipleSelection: false, AllowsOtherFiletypes: true,
    })
    if (path) setSourcePrompt({...sourcePrompt, kind: 'local', location: path, error: ''})
  }

  async function loadWithSource(source: SourceSelectionDTO) {
    if (!sourcePrompt) return
    if (source.kind !== 'none' && !source.location.trim()) {
      setSourcePrompt({...sourcePrompt, error: source.kind === 'git' ? 'Enter a Git repository URI.' : 'Choose a local source folder.'})
      return
    }
    if (!Number.isInteger(source.contextLines) || source.contextLines < 0 || source.contextLines > 20) {
      setSourcePrompt({...sourcePrompt, error: 'Context lines must be a whole number between 0 and 20.'})
      return
    }
    if (source.kind === 'git' && source.gitAuthentication.provider !== 'existing') {
      if ((source.gitAuthentication.provider === 'github' || source.gitAuthentication.provider === 'bitbucket') && !source.gitAuthentication.username.trim()) {
        setSourcePrompt({...sourcePrompt, error: `Enter your ${source.gitAuthentication.provider === 'github' ? 'GitHub' : 'Bitbucket'} username.`})
        return
      }
      if (!source.gitAuthentication.personalAccessToken.trim()) {
        setSourcePrompt({...sourcePrompt, error: 'Enter a personal access token.'})
        return
      }
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
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
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
    const comment = draft.comment.trim()
    if (!comment) {
      setValidationError('Add a comment before applying this review.')
      return
    }
    const review: FindingReview = {
      runIndex: selectedFinding.key.runIndex, resultIndex: selectedFinding.key.resultIndex,
      severity: draft.severity, disposition: draft.disposition, comment, reviewedAt: new Date().toISOString(),
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
      Title: 'Export reviewed SARIF', ButtonText: 'Export', Filename: reviewedFilename(document.fileName),
      CanCreateDirectories: true, AllowsOtherFiletypes: false, Filters: [{DisplayName: 'SARIF files', Pattern: '*.sarif'}],
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

  const reviewedCount = summary.dispositionCounts.confirmed + summary.dispositionCounts['false-positive']
  return (
    <div className="app-shell">
      <header className="titlebar">
        <div className="brand" aria-label="Sastafras"><span className="brand-mark" aria-hidden="true">S</span><span className="brand-name">Sastafras</span><span className="brand-product">SARIF Review</span></div>
        <div className="titlebar-actions">
          {document && <span className={`save-state ${dirty ? 'is-dirty' : ''}`}><i />{dirty ? 'Unexported changes' : 'All changes exported'}</span>}
          <button className="button button-secondary" onClick={openSARIF} disabled={loading}>{icons.folder}Open</button>
          <button className="button button-primary" onClick={exportSARIF} disabled={!document || !dirty}>{icons.export}Export</button>
        </div>
      </header>
      {feedback && <div className={`feedback feedback-${feedback.kind}`} role="status">{feedback.message}<button onClick={() => setFeedback(null)} aria-label="Dismiss message">×</button></div>}
      {sourcePrompt && <SourceImportModal prompt={sourcePrompt} loading={loading} onChange={setSourcePrompt} onChooseFolder={chooseSourceFolder} onCancel={() => setSourcePrompt(null)} onLoad={loadWithSource}/>}
      {!document ? <Welcome loading={loading} onOpen={openSARIF}/> : (
        <main className="workspace">
          <section className="document-heading">
            <div><div className="document-title-row"><span className="document-file-icon">{icons.file}</span><h1>{document.fileName}</h1><span className="version-badge">SARIF {document.version}</span></div><p title={document.sourcePath}>{document.runs?.map((run) => run.toolName).filter((name, index, all) => all.indexOf(name) === index).join(' · ')} · {document.runs?.length ?? 0} {(document.runs?.length ?? 0) === 1 ? 'run' : 'runs'}</p></div>
            <div className="headline-count"><strong>{document.findingCount.toLocaleString()}</strong><span>Total findings</span></div>
          </section>
          {document.snippetSummary && <div className="snippet-summary" role="status"><strong>{document.snippetSummary.generated.toLocaleString()}</strong> generated · <strong>{document.snippetSummary.embedded.toLocaleString()}</strong> embedded · <strong>{document.snippetSummary.unavailable.toLocaleString()}</strong> unavailable</div>}
          <section className="summary-grid" aria-label="Finding summary">
            {severities.map((severity) => <button key={severity} className={`summary-card severity-${severity} ${severityFilter === severity ? 'is-active' : ''}`} onClick={() => setSeverityFilter(severityFilter === severity ? 'all' : severity)}><span>{titleCase(severity)}</span><strong>{summary.severityCounts[severity].toLocaleString()}</strong></button>)}
            <div className="review-progress"><div><span>Reviewed</span><strong>{reviewedCount.toLocaleString()} <small>/ {document.findingCount.toLocaleString()}</small></strong></div><div className="progress-track"><span style={{width: `${document.findingCount ? (reviewedCount / document.findingCount) * 100 : 0}%`}} /></div></div>
          </section>
          <section className="review-layout">
            <aside className="finding-browser" aria-label="Findings">
              <div className="browser-toolbar">
                <label className="search-box">{icons.search}<span className="sr-only">Search findings</span><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search findings…" /></label>
                <div className="filter-row">
                  <label><span className="sr-only">Run</span><select value={runFilter} onChange={(event) => setRunFilter(event.target.value)}><option value="all">All runs</option>{document.runs?.map((run) => <option key={run.index} value={run.index}>{run.name}</option>)}</select></label>
                  <label><span className="sr-only">Severity</span><select value={severityFilter} onChange={(event) => setSeverityFilter(event.target.value)}><option value="all">All severities</option>{severities.map((severity) => <option key={severity} value={severity}>{titleCase(severity)}</option>)}</select></label>
                  <label><span className="sr-only">Disposition</span><select value={dispositionFilter} onChange={(event) => setDispositionFilter(event.target.value)}><option value="all">All statuses</option>{dispositions.map((disposition) => <option key={disposition} value={disposition}>{titleCase(disposition)}</option>)}</select></label>
                </div>
              </div>
              <div className="result-count"><strong>{filteredFindings.length.toLocaleString()}</strong> {filteredFindings.length === 1 ? 'finding' : 'findings'}</div>
              <div className="finding-list">
                {pageFindings.length ? pageFindings.map(({finding, review}) => <FindingRow key={findingID(finding)} finding={finding} review={review} selected={findingID(finding) === selectedID} onSelect={selectFinding}/>) : <div className="no-results"><strong>No matching findings</strong><span>Try changing your search or filters.</span><button onClick={clearFilters}>Clear filters</button></div>}
              </div>
              {filteredFindings.length > PAGE_SIZE && <nav className="pagination" aria-label="Finding pages"><button onClick={() => setPage((value) => Math.max(0, value - 1))} disabled={page === 0} aria-label="Previous page">‹</button><span>Page {page + 1} of {pageCount}</span><button onClick={() => setPage((value) => Math.min(pageCount - 1, value + 1))} disabled={page >= pageCount - 1} aria-label="Next page">›</button></nav>}
            </aside>
            <article className="finding-detail">
              {selectedFinding ? <FindingDetail finding={selectedFinding} draft={draft} setDraft={setDraft} draftDirty={draftDirty} validationError={validationError} setValidationError={setValidationError} appliedReview={reviews.get(selectedID)} onReset={resetDraft} onApply={applyReview}/> : <div className="detail-empty">Select a finding to review its details.</div>}
            </article>
          </section>
        </main>
      )}
    </div>
  )
}

function SourceImportModal({prompt, loading, onChange, onChooseFolder, onCancel, onLoad}: {
  prompt: SourcePrompt
  loading: boolean
  onChange: (prompt: SourcePrompt) => void
  onChooseFolder: () => void
  onCancel: () => void
  onLoad: (source: SourceSelectionDTO) => void
}) {
  const gitAuthentication: GitAuthenticationDTO = prompt.kind === 'git'
    ? {provider: prompt.gitProvider, username: prompt.gitUsername.trim(), personalAccessToken: prompt.gitPersonalAccessToken.trim()}
    : emptyGitAuthentication
  const source: SourceSelectionDTO = {kind: prompt.kind, location: prompt.location.trim(), contextLines: prompt.contextLines, gitAuthentication}
  const gitProviderDetails = {
    existing: {
      placeholder: 'git@host:owner/project.git',
      note: 'Uses public access, SSH agents, or credentials already configured for your system Git client.',
    },
    github: {
      placeholder: 'https://github.com/owner/project.git',
      note: 'Use a GitHub personal access token with read access to the repository. Organization SSO may also need authorization.',
    },
    bitbucket: {
      placeholder: 'https://bitbucket.example.com/scm/project/repository.git',
      note: 'Use a Bitbucket Data Center personal user access token with repository read permission.',
    },
    'azure-devops': {
      placeholder: 'https://dev.azure.com/organization/project/_git/repository',
      note: 'Use an Azure DevOps personal access token with Code read permission.',
    },
  }[prompt.gitProvider]
  const changeSourceKind = (kind: SourceKind) => onChange({...prompt, kind, location: '', gitPersonalAccessToken: '', revealGitPersonalAccessToken: false, error: ''})
  const changeGitProvider = (provider: GitProvider) => onChange({...prompt, gitProvider: provider, gitUsername: '', gitPersonalAccessToken: '', revealGitPersonalAccessToken: false, error: ''})
  return <div className="modal-backdrop"><section className="source-modal" role="dialog" aria-modal="true" aria-labelledby="source-modal-title">
    <div className="modal-header"><p className="eyebrow">Source code</p><h2 id="source-modal-title">Add code context to findings</h2><p>Choose the source used by this scan, or continue with snippets already embedded in the SARIF file.</p></div>
    <div className="source-tabs" role="tablist" aria-label="Source type">
      <button role="tab" aria-selected={prompt.kind === 'local'} className={prompt.kind === 'local' ? 'is-active' : ''} onClick={() => changeSourceKind('local')}>Local folder</button>
      <button role="tab" aria-selected={prompt.kind === 'git'} className={prompt.kind === 'git' ? 'is-active' : ''} onClick={() => changeSourceKind('git')}>Git repository</button>
    </div>
    <div className="source-fields">
      {prompt.kind === 'local' ? <label><span>Source folder</span><div className="folder-input"><input value={prompt.location} readOnly placeholder="No folder selected"/><button className="button button-secondary" onClick={onChooseFolder} disabled={loading}>Browse…</button></div></label> : <>
        <label><span>Git authentication</span><select aria-label="Git authentication" value={prompt.gitProvider} onChange={(event) => changeGitProvider(event.target.value as GitProvider)} disabled={loading}>
          <option value="github">GitHub personal access token</option>
          <option value="bitbucket">Bitbucket Data Center personal access token</option>
          <option value="azure-devops">Azure DevOps personal access token</option>
          <option value="existing">Existing Git authentication</option>
        </select></label>
        <label><span>Git repository URI</span><input value={prompt.location} onChange={(event) => onChange({...prompt, location: event.target.value, error: ''})} placeholder={gitProviderDetails.placeholder} autoFocus autoComplete="off" spellCheck={false}/><small className="source-note">{gitProviderDetails.note}</small></label>
        {(prompt.gitProvider === 'github' || prompt.gitProvider === 'bitbucket') && <label><span>Username</span><input aria-label="Username" value={prompt.gitUsername} onChange={(event) => onChange({...prompt, gitUsername: event.target.value, error: ''})} placeholder={prompt.gitProvider === 'github' ? 'GitHub username' : 'Bitbucket username'} autoComplete="off" spellCheck={false}/></label>}
        {prompt.gitProvider !== 'existing' && <label><span>Personal access token</span><div className="secret-input"><input aria-label="Personal access token" type={prompt.revealGitPersonalAccessToken ? 'text' : 'password'} value={prompt.gitPersonalAccessToken} onChange={(event) => onChange({...prompt, gitPersonalAccessToken: event.target.value, error: ''})} placeholder="Paste token" autoComplete="new-password" spellCheck={false}/><button type="button" className="button button-secondary" aria-label={prompt.revealGitPersonalAccessToken ? 'Hide personal access token' : 'Show personal access token'} onClick={() => onChange({...prompt, revealGitPersonalAccessToken: !prompt.revealGitPersonalAccessToken})} disabled={loading}>{prompt.revealGitPersonalAccessToken ? 'Hide' : 'Show'}</button></div><small className="source-note">Used for this import attempt only. The token is not saved.</small></label>}
      </>}
      <div className="context-field"><label htmlFor="context-lines">Context lines</label><input id="context-lines" type="number" min={0} max={20} step={1} value={prompt.contextLines} onChange={(event) => onChange({...prompt, contextLines: Number(event.target.value), error: ''})}/><small>Lines shown before and after the affected range (0–20).</small></div>
    </div>
    {prompt.error && <div className="modal-error" role="alert">{prompt.error}</div>}
    <div className="modal-actions"><button className="button button-ghost" onClick={onCancel} disabled={loading}>Cancel</button><button className="button button-secondary" onClick={() => onLoad({kind: 'none', location: '', contextLines: prompt.contextLines, gitAuthentication: emptyGitAuthentication})} disabled={loading}>Continue without source</button><button className="button button-primary" onClick={() => onLoad(source)} disabled={loading}>{loading ? <span className="spinner"/> : icons.folder}{loading ? (prompt.kind === 'git' ? 'Cloning…' : 'Reading…') : 'Load with source'}</button></div>
  </section></div>
}

function Welcome({loading, onOpen}: {loading: boolean; onOpen: () => void}) {
  return <main className="welcome"><div className="welcome-graphic" aria-hidden="true"><span className="scan-line"/><span className="document-icon">{icons.file}</span><span className="status-dot dot-one"/><span className="status-dot dot-two"/><span className="status-dot dot-three"/></div><p className="eyebrow">Local security review</p><h1>Turn scan output into<br/><span>decisions you can trust.</span></h1><p className="welcome-copy">Open a SARIF 2.1.0 file to triage findings, adjust security severity, and document every review decision.</p><button className="button button-primary button-large" onClick={onOpen} disabled={loading}>{loading ? <span className="spinner"/> : icons.folder}{loading ? 'Reading SARIF…' : 'Open SARIF file'}</button><div className="privacy-note"><span className="lock-icon" aria-hidden="true">◈</span>Your files and reviews stay on this device.</div></main>
}

function FindingRow({finding, review, selected, onSelect}: {finding: FindingDTO; review: Draft; selected: boolean; onSelect: (finding: FindingDTO) => void}) {
  return <button className={`finding-row ${selected ? 'is-selected' : ''}`} onClick={() => onSelect(finding)}><span className={`severity-bar severity-bg-${review.severity}`}/><span className="finding-content"><span className="finding-topline"><strong>{finding.ruleId || 'Unidentified rule'}</strong><span className={`severity-pill severity-${review.severity}`}>{titleCase(review.severity)}</span></span><span className="finding-message">{finding.message}</span><span className="finding-meta"><span>{icons.location}{locationLabel(finding)}</span><span className={`disposition disposition-${review.disposition}`}>{titleCase(review.disposition)}</span></span></span></button>
}

function FindingDetail({finding, draft, setDraft, draftDirty, validationError, setValidationError, appliedReview, onReset, onApply}: {finding: FindingDTO; draft: Draft; setDraft: (draft: Draft) => void; draftDirty: boolean; validationError: string; setValidationError: (error: string) => void; appliedReview?: FindingReview; onReset: () => void; onApply: () => void}) {
  return <><div className="detail-scroll"><div className="detail-header"><div className="detail-labels"><span className={`severity-pill severity-${draft.severity}`}>{titleCase(draft.severity)}</span><span className={`disposition disposition-${draft.disposition}`}>{titleCase(draft.disposition)}</span></div><h2>{finding.ruleName || finding.ruleId || 'Finding'}</h2><p className="rule-id">{finding.ruleId || 'No rule identifier'} · {finding.toolName} · {finding.runName}</p></div>
    <section className="detail-section"><h3>Finding</h3><p className="finding-description">{finding.message}</p></section>
    {finding.ruleDescription && <section className="detail-section"><h3>Rule</h3><p>{finding.ruleDescription}</p>{finding.helpUri && <button className="text-link" onClick={() => Browser.OpenURL(finding.helpUri)}>View rule guidance {icons.external}</button>}</section>}
    <section className="detail-section"><h3>Location</h3><div className="location-card">{icons.location}<div><strong>{locationLabel(finding)}</strong>{finding.location.startLine > 0 && <span>Line {finding.location.startLine}{finding.location.endLine > finding.location.startLine ? `–${finding.location.endLine}` : ''}</span>}</div></div><SnippetView finding={finding}/></section>
    <section className="detail-section review-section"><div className="section-heading"><div><h3>Review decision</h3><p>Every applied change requires a comment.</p></div>{finding.reviewedAt && !appliedReview && <span className="previous-review">Previously reviewed</span>}</div>
      <fieldset><legend>Security severity</legend><div className="choice-grid severity-choices">{severities.map((severity) => <label key={severity} className={draft.severity === severity ? 'is-selected' : ''}><input type="radio" name="severity" value={severity} checked={draft.severity === severity} onChange={() => setDraft({...draft, severity})}/><i className={`severity-bg-${severity}`}/>{titleCase(severity)}</label>)}</div></fieldset>
      <fieldset><legend>Disposition</legend><div className="choice-grid disposition-choices">{dispositions.map((disposition) => <label key={disposition} className={draft.disposition === disposition ? 'is-selected' : ''}><input type="radio" name="disposition" value={disposition} checked={draft.disposition === disposition} onChange={() => setDraft({...draft, disposition})}/><span aria-hidden="true">{disposition === 'confirmed' ? '✓' : disposition === 'false-positive' ? '×' : '•'}</span>{titleCase(disposition)}</label>)}</div></fieldset>
      <label className="comment-field"><span>Review comment <em>Required</em></span><textarea value={draft.comment} onChange={(event) => {setDraft({...draft, comment: event.target.value}); setValidationError('')}} rows={4} placeholder="Explain the reasoning behind this review decision…" aria-invalid={Boolean(validationError)} aria-describedby={validationError ? 'comment-error' : undefined}/></label>{validationError && <p className="validation-error" id="comment-error" role="alert">{validationError}</p>}
    </section></div><div className="detail-actions"><span>{draftDirty ? 'Unapplied changes' : appliedReview ? `Applied ${new Date(appliedReview.reviewedAt).toLocaleString()}` : 'No pending changes'}</span><div><button className="button button-ghost" onClick={onReset} disabled={!draftDirty}>Reset</button><button className="button button-primary" onClick={onApply} disabled={!draftDirty}>{icons.check}Apply review</button></div></div></>
}

function SnippetView({finding}: {finding: FindingDTO}) {
  const {location} = finding
  if (location.snippetStatus !== 'generated' && location.snippetStatus !== 'embedded') {
    const messages: Record<string, string> = {
      'missing-location': 'This finding does not include a usable source location.',
      'ambiguous': 'The artifact location matches more than one source file.',
      'outside-root': 'The artifact location resolves outside the selected source folder.',
      'binary': 'The matching source file is not UTF-8 text.',
      'too-large': 'The matching source file is too large to preview.',
      'invalid-line': 'The reported line range is outside the matching source file.',
      'unreadable': 'The matching source file could not be read.',
      'not-found': 'No matching source file or embedded snippet is available.',
    }
    return <p className="muted">{messages[location.snippetStatus] ?? messages['not-found']}</p>
  }
  const firstLine = location.snippetStartLine || location.startLine || 1
  const affectedEnd = location.endLine >= location.startLine ? location.endLine : location.startLine
  return <div className="snippet-block"><div className="snippet-heading"><span>{location.snippetOrigin === 'source' ? 'Generated from source' : 'Embedded in SARIF'}</span></div><pre className="code-snippet"><code>{location.snippet.split('\n').map((line, index) => {
    const lineNumber = firstLine + index
    const affected = location.startLine > 0 && lineNumber >= location.startLine && lineNumber <= affectedEnd
    return <span className={`code-line ${affected ? 'is-affected' : ''}`} key={lineNumber}><i>{lineNumber}</i><b>{line || ' '}</b></span>
  })}</code></pre></div>
}

export default App
