import {icons} from './icons'

interface WelcomeProps {
  loading: boolean
  onOpen: () => void
}

export function Welcome({loading, onOpen}: WelcomeProps) {
  return <main className="welcome"><div className="welcome-graphic" aria-hidden="true"><span className="scan-line"/><span className="document-icon">{icons.file}</span><span className="status-dot dot-one"/><span className="status-dot dot-two"/><span className="status-dot dot-three"/></div><p className="eyebrow">Local security review</p><h1>Turn scan output into<br/><span>decisions you can trust.</span></h1><p className="welcome-copy">Open a SARIF 2.1.0 file to triage findings, adjust security severity, and document every review decision.</p><button className="button button-primary button-large" onClick={onOpen} disabled={loading}>{loading ? <span className="spinner"/> : icons.folder}{loading ? 'Reading SARIF…' : 'Open SARIF file'}</button><div className="privacy-note"><span className="lock-icon" aria-hidden="true">◈</span>Your files and reviews stay on this device.</div></main>
}
