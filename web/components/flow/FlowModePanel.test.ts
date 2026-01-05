import { describe, expect, it } from "vitest";

import { parseLlmSuggestions } from "./FlowModePanel";

describe("parseLlmSuggestions", () => {
  it("parses structured JSON and sorts by priority", () => {
    const result = parseLlmSuggestions(
      JSON.stringify({
        prompts: [
          { title: "Second", content: "B", priority: 2 },
          { title: "First", content: "A", priority: 1 },
        ],
        searches: [{ query: "alpha", priority: 3 }],
      }),
    );

    expect(result).not.toBeNull();
    expect(result?.prompts.map((prompt) => prompt.title)).toEqual(["First", "Second"]);
    expect(result?.searches[0]).toMatchObject({ query: "alpha", priority: 3 });
  });

  it("extracts JSON embedded in surrounding text", () => {
    const result = parseLlmSuggestions(
      [
        "Summary:",
        "Here are the next steps.",
        '{"prompts":[{"title":"Rewrite intro","content":"收紧前两段的铺陈"}],"searches":[{"query":"叙事写法 示例"}]}',
      ].join("\n"),
    );

    expect(result).not.toBeNull();
    expect(result?.prompts[0]).toMatchObject({ title: "Rewrite intro", priority: 1 });
    expect(result?.searches[0]).toMatchObject({ query: "叙事写法 示例", priority: 1 });
  });

  it("coerces single objects into arrays when present", () => {
    const result = parseLlmSuggestions(
      '{"prompts":{"title":"Solo","content":"Only one"},"searches":{"query":"learning by writing","reason":"find cases"}}',
    );

    expect(result).not.toBeNull();
    expect(result?.prompts).toHaveLength(1);
    expect(result?.prompts[0]).toMatchObject({ title: "Solo", content: "Only one", priority: 1 });
    expect(result?.searches[0]).toMatchObject({ query: "learning by writing", priority: 1 });
  });
});
