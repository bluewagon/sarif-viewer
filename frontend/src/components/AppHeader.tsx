import type {SARIFDocumentDTO} from '../../bindings/sarif-viewer/backend/models'
import {icons} from './icons'

interface AppHeaderProps {
  document: SARIFDocumentDTO | null
  dirty: boolean
  loading: boolean
  onOpen: () => void
  onExport: () => void
}

export function AppHeader({document, dirty, loading, onOpen, onExport}: AppHeaderProps) {
  return <header className="titlebar">
    <div className="brand" aria-label="SARIF Viewer"><span className="brand-mark" aria-hidden="true">S</span><span className="brand-name">SARIF Viewer</span><span className="brand-product">SARIF Review</span></div>
    <div className="titlebar-actions">
      {document && <span className={`save-state ${dirty ? 'is-dirty' : ''}`}><i />{dirty ? 'Unexported changes' : 'All changes exported'}</span>}
      <button className="button button-secondary" onClick={onOpen} disabled={loading}>{icons.folder}Open</button>
      <button className="button button-primary" onClick={onExport} disabled={!document || !dirty}>{icons.export}Export</button>
    </div>
  </header>
}
