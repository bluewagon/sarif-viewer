import type {Feedback} from '../features/review/model'

interface FeedbackBannerProps {
  feedback: Exclude<Feedback, null>
  onDismiss: () => void
}

export function FeedbackBanner({feedback, onDismiss}: FeedbackBannerProps) {
  return <div className={`feedback feedback-${feedback.kind}`} role="status">{feedback.message}<button onClick={onDismiss} aria-label="Dismiss message">×</button></div>
}
