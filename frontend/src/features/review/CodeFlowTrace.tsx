import type {CodeFlowDTO, CodeFlowStepDTO, ThreadFlowDTO} from '../../../bindings/sarif-viewer/backend/models'
import {physicalLocationLabel} from './model'

interface CodeFlowTraceProps {
  codeFlows: CodeFlowDTO[]
}

function Step({step, index}: {step: CodeFlowStepDTO; index: number}) {
  const location = physicalLocationLabel(step.location)
  const hasExecutionOrder = step.executionOrder >= 0
  const nestingLevel = Number.isFinite(step.nestingLevel) ? Math.min(Math.max(step.nestingLevel, 0), 8) : 0
  return <li className="code-flow-step">
    <span className="code-flow-marker" aria-hidden="true">{hasExecutionOrder ? step.executionOrder : index + 1}</span>
    <div className="code-flow-step-body" style={{marginLeft: `${nestingLevel * 14}px`}}>
      <div className="code-flow-step-heading"><strong>Step {index + 1}</strong>{hasExecutionOrder && <span>Execution order {step.executionOrder}</span>}</div>
      {step.message && <p className="code-flow-step-message">{step.message}</p>}
      {location && <code className="code-flow-location">{location}</code>}
      {!step.message && !location && <p className="muted">No location or message provided.</p>}
    </div>
  </li>
}

function ThreadFlow({thread, index}: {thread: ThreadFlowDTO; index: number}) {
  const title = thread.message || thread.id || `Thread flow ${index + 1}`
  return <section className="thread-flow" aria-label={title}>
    <div className="thread-flow-heading"><strong>{title}</strong>{thread.id && thread.message && <code>{thread.id}</code>}</div>
    {thread.steps?.length
      ? <ol className="code-flow-steps">{thread.steps.map((step, stepIndex) => <Step key={stepIndex} step={step} index={stepIndex}/>)}</ol>
      : <p className="muted">No steps provided.</p>}
  </section>
}

export function CodeFlowTrace({codeFlows}: CodeFlowTraceProps) {
  return <div className="code-flow-list">{codeFlows.map((flow, flowIndex) => {
    const title = flow.message || `Code flow ${flowIndex + 1}`
    return <article className="code-flow-card" key={flowIndex} aria-label={title}>
      <header><span>Flow {flowIndex + 1}</span><h4>{title}</h4></header>
      {flow.threadFlows?.length
        ? flow.threadFlows.map((thread, threadIndex) => <ThreadFlow key={threadIndex} thread={thread} index={threadIndex}/>)
        : <p className="muted">No thread flows provided.</p>}
    </article>
  })}</div>
}
