import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, render, screen, waitFor, within} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import App from './App'
import type {FindingDTO, SARIFDocumentDTO} from '../bindings/sastafras/backend/models'

const mocks = vi.hoisted(() => ({
  openFile: vi.fn(),
  saveFile: vi.fn(),
  question: vi.fn(),
  openURL: vi.fn(),
  loadSARIF: vi.fn(),
  exportSARIF: vi.fn(),
}))

vi.mock('@wailsio/runtime', () => ({
  Dialogs: {OpenFile: mocks.openFile, SaveFile: mocks.saveFile, Question: mocks.question},
  Browser: {OpenURL: mocks.openURL},
}))

vi.mock('../bindings/sastafras/backend', () => ({
  SARIFService: {LoadSARIF: mocks.loadSARIF, ExportSARIF: mocks.exportSARIF},
}))

function finding(index: number, overrides: Partial<FindingDTO> = {}): FindingDTO {
  return {
    key: {runIndex: index % 2, resultIndex: index},
    runName: index % 2 ? 'Secondary run' : 'Primary run',
    toolName: 'Test Scanner',
    ruleId: `RULE-${index}`,
    ruleName: `Rule ${index}`,
    ruleDescription: 'A test security rule.',
    helpUri: 'https://example.test/rule',
    message: `Finding message ${index}`,
    sarifLevel: 'warning',
    severity: index % 2 ? 'high' : 'medium',
    disposition: 'unreviewed',
    comment: '',
    reviewedAt: '',
    location: {uri: `src/file-${index}.ts`, startLine: index + 1, startColumn: 2, endLine: index + 1, endColumn: 8, snippet: 'unsafe(value)'},
    ...overrides,
  }
}

function sarifDocument(count = 2): SARIFDocumentDTO {
  return {
    documentId: 'document-id',
    fileName: 'scan.sarif',
    sourcePath: '/tmp/scan.sarif',
    version: '2.1.0',
    findingCount: count,
    runs: [
      {index: 0, name: 'Primary run', toolName: 'Test Scanner', findingCount: Math.ceil(count / 2)},
      {index: 1, name: 'Secondary run', toolName: 'Test Scanner', findingCount: Math.floor(count / 2)},
    ],
    findings: Array.from({length: count}, (_, index) => finding(index)),
  }
}

async function openDocument(document = sarifDocument()) {
  mocks.openFile.mockResolvedValue('/tmp/scan.sarif')
  mocks.loadSARIF.mockResolvedValue(document)
  await userEvent.click(screen.getByRole('button', {name: 'Open SARIF file'}))
  await screen.findByRole('heading', {name: 'scan.sarif'})
}

function resultCount(value: string) {
  return screen.getByText((_, element) => element?.classList.contains('result-count') === true && element.textContent === value)
}

describe('Sastafras review workspace', () => {
  afterEach(cleanup)

  beforeEach(() => {
    Object.values(mocks).forEach((mock) => mock.mockReset())
    mocks.openFile.mockResolvedValue('')
    mocks.saveFile.mockResolvedValue('')
    mocks.question.mockResolvedValue('Keep reviewing')
    mocks.exportSARIF.mockResolvedValue(undefined)
  })

  it('shows the private local-review welcome state and handles a cancelled picker', async () => {
    render(<App/>)
    expect(screen.getByRole('heading', {name: /Turn scan output intodecisions/})).toBeVisible()
    expect(screen.getByText('Your files and reviews stay on this device.')).toBeVisible()
    await userEvent.click(screen.getByRole('button', {name: 'Open SARIF file'}))
    expect(mocks.openFile).toHaveBeenCalledOnce()
    expect(mocks.loadSARIF).not.toHaveBeenCalled()
  })

  it('shows a parse error without leaving the welcome state', async () => {
    render(<App/>)
    mocks.openFile.mockResolvedValue('/tmp/bad.sarif')
    mocks.loadSARIF.mockRejectedValue(new Error('unsupported SARIF version'))
    await userEvent.click(screen.getByRole('button', {name: 'Open SARIF file'}))
    expect(await screen.findByRole('alert')).toHaveTextContent('unsupported SARIF version')
    expect(screen.getByRole('heading', {name: /Turn scan output/})).toBeVisible()
  })

  it('loads findings and filters by text, run, severity, and disposition', async () => {
    render(<App/>)
    await openDocument()
    expect(resultCount('2 findings')).toBeVisible()
    await userEvent.type(screen.getByPlaceholderText('Search findings…'), 'RULE-1')
    expect(resultCount('1 finding')).toBeVisible()
    expect(screen.getByText('Finding message 1')).toBeVisible()
    await userEvent.clear(screen.getByPlaceholderText('Search findings…'))
    await userEvent.selectOptions(screen.getByRole('combobox', {name: 'Run'}), '1')
    expect(resultCount('1 finding')).toBeVisible()
    await userEvent.selectOptions(screen.getByRole('combobox', {name: 'Severity'}), 'medium')
    expect(screen.getByText('No matching findings')).toBeVisible()
    await userEvent.click(screen.getByRole('button', {name: 'Clear filters'}))
    expect(resultCount('2 findings')).toBeVisible()
  })

  it('requires a comment, applies a review, and replaces it on a later edit', async () => {
    render(<App/>)
    await openDocument()
    await userEvent.click(screen.getByRole('radio', {name: 'Confirmed'}))
    await userEvent.click(screen.getByRole('button', {name: 'Apply review'}))
    expect(screen.getByRole('alert')).toHaveTextContent('Add a comment')
    await userEvent.type(screen.getByPlaceholderText(/Explain the reasoning/), 'Verified data flow')
    await userEvent.click(screen.getByRole('button', {name: 'Apply review'}))
    expect(await screen.findByText('Unexported changes')).toBeVisible()
    expect(screen.getAllByText('Confirmed').length).toBeGreaterThan(0)

    const comment = screen.getByPlaceholderText(/Explain the reasoning/)
    await userEvent.clear(comment)
    await userEvent.type(comment, 'Updated reasoning')
    await userEvent.click(screen.getByRole('button', {name: 'Apply review'}))
    expect(comment).toHaveValue('Updated reasoning')
  })

  it('exports all applied reviews using the reviewed filename', async () => {
    render(<App/>)
    await openDocument()
    await userEvent.click(screen.getByRole('radio', {name: 'False Positive'}))
    await userEvent.type(screen.getByPlaceholderText(/Explain the reasoning/), 'Test fixture only')
    await userEvent.click(screen.getByRole('button', {name: 'Apply review'}))
    mocks.saveFile.mockResolvedValue('/tmp/scan.reviewed.sarif')
    await userEvent.click(screen.getByRole('button', {name: 'Export'}))
    await waitFor(() => expect(mocks.exportSARIF).toHaveBeenCalledOnce())
    expect(mocks.saveFile.mock.calls[0][0].Filename).toBe('scan.reviewed.sarif')
    const [documentID, destination, reviews] = mocks.exportSARIF.mock.calls[0]
    expect(documentID).toBe('document-id')
    expect(destination).toBe('/tmp/scan.reviewed.sarif')
    expect(reviews).toEqual([expect.objectContaining({disposition: 'false-positive', comment: 'Test fixture only'})])
    expect(await screen.findByText('All changes exported')).toBeVisible()
  })

  it('paginates large reports and protects an unapplied draft when changing findings', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    render(<App/>)
    await openDocument(sarifDocument(101))
    expect(screen.getByText('Page 1 of 2')).toBeVisible()
    await userEvent.click(screen.getByRole('radio', {name: 'Confirmed'}))
    const rows = screen.getAllByRole('button').filter((button) => button.classList.contains('finding-row'))
    await userEvent.click(rows[1])
    expect(confirm).toHaveBeenCalledOnce()
    expect(screen.getByRole('heading', {name: 'Rule 0'})).toBeVisible()
    await userEvent.click(screen.getByRole('button', {name: 'Next page'}))
    expect(screen.getByText('Page 2 of 2')).toBeVisible()
    expect(within(screen.getByText('Finding message 100').closest('button')!).getByText('RULE-100')).toBeVisible()
    confirm.mockRestore()
  })

  it('keeps the loaded report when a replacement file fails to parse', async () => {
    render(<App/>)
    await openDocument()
    mocks.openFile.mockResolvedValue('/tmp/bad.sarif')
    mocks.loadSARIF.mockRejectedValue(new Error('invalid SARIF file'))
    await userEvent.click(screen.getByRole('button', {name: 'Open'}))
    expect(await screen.findByRole('status')).toHaveTextContent('invalid SARIF file')
    expect(screen.getByRole('heading', {name: 'scan.sarif'})).toBeVisible()
  })
})
