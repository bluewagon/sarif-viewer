import type {FindingDTO} from '../../../bindings/sastafras/backend/models'

interface SnippetViewProps {
  finding: FindingDTO
}

const unavailableSnippetMessages: Record<string, string> = {
  'missing-location': 'This finding does not include a usable source location.',
  'ambiguous': 'The artifact location matches more than one source file.',
  'outside-root': 'The artifact location resolves outside the selected source folder.',
  'binary': 'The matching source file is not UTF-8 text.',
  'too-large': 'The matching source file is too large to preview.',
  'invalid-line': 'The reported line range is outside the matching source file.',
  'unreadable': 'The matching source file could not be read.',
  'not-found': 'No matching source file or embedded snippet is available.',
}

export function SnippetView({finding}: SnippetViewProps) {
  const {location} = finding
  if (location.snippetStatus !== 'generated' && location.snippetStatus !== 'embedded') {
    return <p className="muted">{unavailableSnippetMessages[location.snippetStatus] ?? unavailableSnippetMessages['not-found']}</p>
  }

  const firstLine = location.snippetStartLine || location.startLine || 1
  const affectedEnd = location.endLine >= location.startLine ? location.endLine : location.startLine
  return <div className="snippet-block"><div className="snippet-heading"><span>{location.snippetOrigin === 'source' ? 'Generated from source' : 'Embedded in SARIF'}</span></div><pre className="code-snippet"><code>{location.snippet.split('\n').map((line, index) => {
    const lineNumber = firstLine + index
    const affected = location.startLine > 0 && lineNumber >= location.startLine && lineNumber <= affectedEnd
    return <span className={`code-line ${affected ? 'is-affected' : ''}`} key={lineNumber}><i>{lineNumber}</i><b>{line || ' '}</b></span>
  })}</code></pre></div>
}
