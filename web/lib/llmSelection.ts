import type { LLMSelection } from "@/lib/types";

const STORAGE_KEY = "alex-llm-selection";

export function loadLLMSelection(): LLMSelection | null {
  if (typeof window === "undefined") return null;
  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as LLMSelection;
  } catch {
    return null;
  }
}

export function saveLLMSelection(selection: LLMSelection) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(selection));
}

export function clearLLMSelection() {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem(STORAGE_KEY);
}
