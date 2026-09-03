import type {FindingDTO, FindingReview} from '../../../bindings/sastafras/backend/models'
import {icons} from '../../components/icons'
import {dispositions, locationLabel, severities, titleCase, type Draft} from './model'
import {SnippetView} from './SnippetView'

interface FindingDetailProps {
  finding: FindingDTO
  draft: Draft
  draftDirty: boolean
  validationError: string
  appliedReview?: FindingReview
  onDraftChange: (draft: Draft) => void
  onValidationErrorChange: (error: string) => void
  onOpenHelp: (uri: string) => void
  onReset: () => void
  onApply: () => void
}

export function FindingDetail({
  finding,
  draft,
  draftDirty,
  validationError,
  appliedReview,
  onDraftChange,
  onValidationErrorChange,
  onOpenHelp,
  onReset,
  onApply,
}: FindingDetailProps) {
  return <>
    <div className="detail-scroll"><div className="detail-header"><div className="detail-labels"><span className={`severity-pill severity-${draft.severity}`}>{titleCase(draft.severity)}</span><span className={`disposition disposition-${draft.disposition}`}>{titleCase(draft.disposition)}</span></div><h2>{finding.ruleName || finding.ruleId || 'Finding'}</h2><p className="rule-id">{finding.ruleId || 'No rule identifier'} · {finding.toolName} · {finding.runName}</p></div>
      <section className="detail-section"><h3>Finding</h3><p className="finding-description">{finding.message}</p></section>
      {finding.ruleDescription && <section className="detail-section"><h3>Rule</h3><p>{finding.ruleDescription}</p>{finding.helpUri && <button className="text-link" onClick={() => onOpenHelp(finding.helpUri)}>View rule guidance {icons.external}</button>}</section>}
      <section className="detail-section"><h3>Location</h3><div className="location-card">{icons.location}<div><strong>{locationLabel(finding)}</strong>{finding.location.startLine > 0 && <span>Line {finding.location.startLine}{finding.location.endLine > finding.location.startLine ? `–${finding.location.endLine}` : ''}</span>}</div></div><SnippetView finding={finding}/></section>
      <section className="detail-section review-section"><div className="section-heading"><div><h3>Review decision</h3><p>Every applied change requires a comment.</p></div>{finding.reviewedAt && !appliedReview && <span className="previous-review">Previously reviewed</span>}</div>
        <fieldset><legend>Security severity</legend><div className="choice-grid severity-choices">{severities.map((severity) => <label key={severity} className={draft.severity === severity ? 'is-selected' : ''}><input type="radio" name="severity" value={severity} checked={draft.severity === severity} onChange={() => onDraftChange({...draft, severity})}/><i className={`severity-bg-${severity}`}/>{titleCase(severity)}</label>)}</div></fieldset>
        <fieldset><legend>Disposition</legend><div className="choice-grid disposition-choices">{dispositions.map((disposition) => <label key={disposition} className={draft.disposition === disposition ? 'is-selected' : ''}><input type="radio" name="disposition" value={disposition} checked={draft.disposition === disposition} onChange={() => onDraftChange({...draft, disposition})}/><span aria-hidden="true">{disposition === 'confirmed' ? '✓' : disposition === 'false-positive' ? '×' : '•'}</span>{titleCase(disposition)}</label>)}</div></fieldset>
        <label className="comment-field"><span>Review comment <em>Required</em></span><textarea value={draft.comment} onChange={(event) => {onDraftChange({...draft, comment: event.target.value}); onValidationErrorChange('')}} rows={4} placeholder="Explain the reasoning behind this review decision…" aria-invalid={Boolean(validationError)} aria-describedby={validationError ? 'comment-error' : undefined}/></label>{validationError && <p className="validation-error" id="comment-error" role="alert">{validationError}</p>}
      </section>
    </div>
    <div className="detail-actions"><span>{draftDirty ? 'Unapplied changes' : appliedReview ? `Applied ${new Date(appliedReview.reviewedAt).toLocaleString()}` : 'No pending changes'}</span><div><button className="button button-ghost" onClick={onReset} disabled={!draftDirty}>Reset</button><button className="button button-primary" onClick={onApply} disabled={!draftDirty}>{icons.check}Apply review</button></div></div>
  </>
}
