import type {GitAuthenticationDTO, SourceSelectionDTO} from '../../../bindings/sarif-viewer/backend/models'

export type SourceKind = 'local' | 'git'
export type GitProvider = 'existing' | 'github' | 'bitbucket' | 'azure-devops'

export interface SourcePrompt {
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

export const emptyGitAuthentication: GitAuthenticationDTO = {provider: '', username: '', personalAccessToken: ''}

export function createSourcePrompt(sarifPath: string): SourcePrompt {
  return {
    sarifPath,
    kind: 'local',
    location: '',
    contextLines: 3,
    gitProvider: 'github',
    gitUsername: '',
    gitPersonalAccessToken: '',
    revealGitPersonalAccessToken: false,
    error: '',
  }
}

export function sourceSelectionFromPrompt(prompt: SourcePrompt): SourceSelectionDTO {
  const gitAuthentication: GitAuthenticationDTO = prompt.kind === 'git'
    ? {provider: prompt.gitProvider, username: prompt.gitUsername.trim(), personalAccessToken: prompt.gitPersonalAccessToken.trim()}
    : emptyGitAuthentication
  return {kind: prompt.kind, location: prompt.location.trim(), contextLines: prompt.contextLines, gitAuthentication}
}

export function validateSourceSelection(source: SourceSelectionDTO): string | null {
  if (source.kind !== 'none' && !source.location.trim()) {
    return source.kind === 'git' ? 'Enter a Git repository URI.' : 'Choose a local source folder.'
  }
  if (!Number.isInteger(source.contextLines) || source.contextLines < 0 || source.contextLines > 20) {
    return 'Context lines must be a whole number between 0 and 20.'
  }
  if (source.kind === 'git' && source.gitAuthentication.provider !== 'existing') {
    if ((source.gitAuthentication.provider === 'github' || source.gitAuthentication.provider === 'bitbucket') && !source.gitAuthentication.username.trim()) {
      return `Enter your ${source.gitAuthentication.provider === 'github' ? 'GitHub' : 'Bitbucket'} username.`
    }
    if (!source.gitAuthentication.personalAccessToken.trim()) return 'Enter a personal access token.'
  }
  return null
}
