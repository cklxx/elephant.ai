import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { LLMIndicator } from "@/components/agent/LLMIndicator";

vi.mock("@/lib/api", () => ({
  getRuntimeConfigSnapshot: vi.fn().mockResolvedValue({
    effective: { llm_provider: "codex", llm_model: "model-a" },
    overrides: {},
    sources: { api_key: "codex_cli", llm_model: "file" },
  }),
  getSubscriptionCatalog: vi.fn().mockResolvedValue({
    providers: [{ provider: "codex", source: "codex_cli", models: ["model-a"] }],
  }),
}));

describe("LLMIndicator", () => {
  it("stores selection locally when a model is chosen", async () => {
    localStorage.removeItem("alex-llm-selection");
    render(<LLMIndicator />);
    const trigger = await screen.findByRole("button", { name: /llm/i });
    await userEvent.click(trigger);
    await userEvent.click(await screen.findByRole("menuitem", { name: /model-a/i }));
    const stored = JSON.parse(localStorage.getItem("alex-llm-selection") ?? "{}");
    expect(stored.model).toBe("model-a");
  });
});
