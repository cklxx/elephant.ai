export const AnalyticsEvent = {
  TaskSubmitted: 'task_submitted',
  TaskSubmissionFailed: 'task_submission_failed',
  TaskRetriedWithoutSession: 'task_retried_without_session',
  TaskCancelRequested: 'task_cancel_requested',
  TaskCancelFailed: 'task_cancel_failed',
  SessionSelected: 'session_selected',
  SessionCreated: 'session_created',
  SessionDeleted: 'session_deleted',
  SidebarToggled: 'sidebar_toggled',
  TimelineViewed: 'timeline_viewed',
  AutoReviewContinue: 'auto_review_continue',
  AutoReviewContinueWithNotes: 'auto_review_continue_with_notes',
} as const;

export type AnalyticsEventName = (typeof AnalyticsEvent)[keyof typeof AnalyticsEvent];
