import { describe, expect, it, vi } from "vitest";

import { getOnboardingState, updateOnboardingState } from "@/lib/api";

vi.mock("@/lib/api-base", () => ({ buildApiUrl: (path: string) => path }));

global.fetch = vi.fn().mockResolvedValue({
  ok: true,
  status: 200,
  headers: new Headers(),
  json: async () => ({ state: {}, completed: false }),
}) as any;

describe("onboarding state api", () => {
  it("loads onboarding state", async () => {
    const result = await getOnboardingState();
    expect(result.completed).toBe(false);
    expect(fetch).toHaveBeenCalledWith(
      "/api/internal/onboarding/state",
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("updates onboarding state", async () => {
    await updateOnboardingState({
      state: {
        selected_provider: "codex",
        selected_model: "gpt-5.2-codex",
        used_source: "codex_cli",
      },
    });
    expect(fetch).toHaveBeenCalledWith(
      "/api/internal/onboarding/state",
      expect.objectContaining({
        method: "PUT",
        credentials: "include",
      }),
    );
  });
});

