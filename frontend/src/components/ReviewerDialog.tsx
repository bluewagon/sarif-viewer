import {useRef, type FormEvent, type KeyboardEvent} from 'react'

interface ReviewerDialogProps {
  value: string
  error: string
  canCancel: boolean
  onChange: (value: string) => void
  onConfirm: () => void
  onCancel: () => void
}

export function ReviewerDialog({value, error, canCancel, onChange, onConfirm, onCancel}: ReviewerDialogProps) {
  const dialogRef = useRef<HTMLElement>(null)

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    onConfirm()
  }

  function handleKeyDown(event: KeyboardEvent<HTMLElement>) {
    if (event.key === 'Escape' && canCancel) {
      event.preventDefault()
      onCancel()
      return
    }
    if (event.key !== 'Tab') return
    const focusable = Array.from(dialogRef.current?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled)') ?? [])
    if (!focusable.length) return
    const first = focusable[0]
    const last = focusable[focusable.length - 1]
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }

  return <div className="modal-backdrop">
    <section ref={dialogRef} className="source-modal reviewer-modal" role="dialog" aria-modal="true" aria-labelledby="reviewer-modal-title" aria-describedby="reviewer-modal-description" onKeyDown={handleKeyDown}>
      <form onSubmit={submit}>
        <div className="modal-header">
          <p className="eyebrow">Reviewer identity</p>
          <h2 id="reviewer-modal-title">Who is reviewing these findings?</h2>
          <p id="reviewer-modal-description">Your username will be recorded with each review you apply and included in the exported SARIF file.</p>
        </div>
        <div className="source-fields">
          <label htmlFor="reviewer-username"><span>Username</span><input id="reviewer-username" value={value} onChange={(event) => onChange(event.target.value)} autoComplete="username" autoFocus aria-invalid={Boolean(error)} aria-describedby={error ? 'reviewer-error' : undefined}/></label>
        </div>
        {error && <div className="modal-error" id="reviewer-error" role="alert">{error}</div>}
        <div className="modal-actions">
          {canCancel && <button type="button" className="button button-ghost" onClick={onCancel}>Cancel</button>}
          <button type="submit" className="button button-primary">Continue as reviewer</button>
        </div>
      </form>
    </section>
  </div>
}
