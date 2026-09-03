import {describe, expect, it} from 'vitest'
import {sourceSelectionFromPrompt, validateSourceSelection, type SourcePrompt} from './model'

function prompt(overrides: Partial<SourcePrompt> = {}): SourcePrompt {
  return {
    sarifPath: '/tmp/scan.sarif',
    kind: 'local',
    location: '/tmp/project',
    contextLines: 3,
    gitProvider: 'github',
    gitUsername: '',
    gitPersonalAccessToken: '',
    revealGitPersonalAccessToken: false,
    error: '',
    ...overrides,
  }
}

describe('source selection', () => {
  it('builds a trimmed Git source selection from prompt state', () => {
    const selection = sourceSelectionFromPrompt(prompt({
      kind: 'git',
      location: ' https://github.com/example/project.git ',
      gitUsername: ' octocat ',
      gitPersonalAccessToken: ' token ',
    }))

    expect(selection).toEqual({
      kind: 'git',
      location: 'https://github.com/example/project.git',
      contextLines: 3,
      gitAuthentication: {provider: 'github', username: 'octocat', personalAccessToken: 'token'},
    })
  })

  it('validates source location, context range, and token credentials', () => {
    expect(validateSourceSelection(sourceSelectionFromPrompt(prompt({location: ''})))).toBe('Choose a local source folder.')
    expect(validateSourceSelection(sourceSelectionFromPrompt(prompt({contextLines: 21})))).toBe('Context lines must be a whole number between 0 and 20.')
    expect(validateSourceSelection(sourceSelectionFromPrompt(prompt({kind: 'git', location: 'https://github.com/example/project.git'})))).toBe('Enter your GitHub username.')
    expect(validateSourceSelection(sourceSelectionFromPrompt(prompt({kind: 'git', location: 'https://github.com/example/project.git', gitUsername: 'octocat'})))).toBe('Enter a personal access token.')
  })
})
