import type {SourceSelectionDTO} from '../../../bindings/sarif-viewer/backend/models'
import {icons} from '../../components/icons'
import {
  emptyGitAuthentication,
  sourceSelectionFromPrompt,
  type GitProvider,
  type SourceKind,
  type SourcePrompt,
} from './model'

interface SourceImportModalProps {
  prompt: SourcePrompt
  loading: boolean
  onChange: (prompt: SourcePrompt) => void
  onChooseFolder: () => void
  onCancel: () => void
  onLoad: (source: SourceSelectionDTO) => void
}

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
}

export function SourceImportModal({prompt, loading, onChange, onChooseFolder, onCancel, onLoad}: SourceImportModalProps) {
  const source = sourceSelectionFromPrompt(prompt)
  const providerDetails = gitProviderDetails[prompt.gitProvider]
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
        <label><span>Git repository URI</span><input value={prompt.location} onChange={(event) => onChange({...prompt, location: event.target.value, error: ''})} placeholder={providerDetails.placeholder} autoFocus autoComplete="off" spellCheck={false}/><small className="source-note">{providerDetails.note}</small></label>
        {(prompt.gitProvider === 'github' || prompt.gitProvider === 'bitbucket') && <label><span>Username</span><input aria-label="Username" value={prompt.gitUsername} onChange={(event) => onChange({...prompt, gitUsername: event.target.value, error: ''})} placeholder={prompt.gitProvider === 'github' ? 'GitHub username' : 'Bitbucket username'} autoComplete="off" spellCheck={false}/></label>}
        {prompt.gitProvider !== 'existing' && <label><span>Personal access token</span><div className="secret-input"><input aria-label="Personal access token" type={prompt.revealGitPersonalAccessToken ? 'text' : 'password'} value={prompt.gitPersonalAccessToken} onChange={(event) => onChange({...prompt, gitPersonalAccessToken: event.target.value, error: ''})} placeholder="Paste token" autoComplete="new-password" spellCheck={false}/><button type="button" className="button button-secondary" aria-label={prompt.revealGitPersonalAccessToken ? 'Hide personal access token' : 'Show personal access token'} onClick={() => onChange({...prompt, revealGitPersonalAccessToken: !prompt.revealGitPersonalAccessToken})} disabled={loading}>{prompt.revealGitPersonalAccessToken ? 'Hide' : 'Show'}</button></div><small className="source-note">Used for this import attempt only. The token is not saved.</small></label>}
      </>}
      <div className="context-field"><label htmlFor="context-lines">Context lines</label><input id="context-lines" type="number" min={0} max={20} step={1} value={prompt.contextLines} onChange={(event) => onChange({...prompt, contextLines: Number(event.target.value), error: ''})}/><small>Lines shown before and after the affected range (0–20).</small></div>
    </div>
    {prompt.error && <div className="modal-error" role="alert">{prompt.error}</div>}
    <div className="modal-actions"><button className="button button-ghost" onClick={onCancel} disabled={loading}>Cancel</button><button className="button button-secondary" onClick={() => onLoad({kind: 'none', location: '', contextLines: prompt.contextLines, gitAuthentication: emptyGitAuthentication})} disabled={loading}>Continue without source</button><button className="button button-primary" onClick={() => onLoad(source)} disabled={loading}>{loading ? <span className="spinner"/> : icons.folder}{loading ? (prompt.kind === 'git' ? 'Cloning…' : 'Reading…') : 'Load with source'}</button></div>
  </section></div>
}
