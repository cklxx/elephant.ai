export interface OnboardingState {
  completed_at?: string;
  selected_provider?: string;
  selected_model?: string;
  used_source?: string;
  advanced_overrides_used?: boolean;
}

export interface OnboardingStateResponse {
  state: OnboardingState;
  completed: boolean;
}

export interface OnboardingStateUpdatePayload {
  state: OnboardingState;
}

