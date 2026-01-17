import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { LLMIndicator } from "@/components/agent/LLMIndicator";

vi.mock("@/lib/api", () => ({
  getRuntimeConfigSnapshot: vi.fn().mockResolvedValue({
    effective: { llm_provider: "codex", llm_model: "model-a" },
    overrides: {},
    sources: { api_key: "codex_cli" },
  }),
  updateRuntimeConfig: vi.fn().mockResolvedValue({
    effective: { llm_provider: "codex", llm_model: "model-a" },
    overrides: {},
    sources: { api_key: "codex_cli" },
  }),
  getRuntimeModelCatalog: vi.fn().mockResolvedValue({
    providers: [{ provider: "codex", source: "codex_cli", models: ["model-a"] }],
  }),
}));

describe("LLMIndicator", () => {
  it("loads CLI models when opened", async () => {
    render(<LLMIndicator />);
    const trigger = await screen.findByRole("button", { name: /llm/i });
    await userEvent.click(trigger);
    expect(
      await screen.findByRole("menuitem", { name: /model-a/i }),
    ).toBeInTheDocument();
  });
});
