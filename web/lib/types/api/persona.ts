export interface UserPersonaDrive {
  id: string;
  label: string;
  score: number;
}

export interface UserPersonaGoals {
  current_focus?: string;
  one_year?: string;
  three_year?: string;
}

export interface UserPersonaProfile {
  version: string;
  updated_at: string;
  initiative_sources?: string[];
  core_drives?: UserPersonaDrive[];
  top_drives?: string[];
  values?: string[];
  goals?: UserPersonaGoals;
  traits?: Record<string, number>;
  decision_style?: string;
  risk_profile?: string;
  conflict_style?: string;
  key_choices?: string[];
  non_negotiables?: string;
  summary?: string;
  construction_rules?: string[];
  raw_answers?: Record<string, unknown>;
}
