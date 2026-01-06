import { describe, expect, it } from "vitest";

import { buildWriteActionPrompt, parseLlmSuggestions } from "./FlowModePanel";

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

  it("falls back to textual parsing when JSON is absent", () => {
    const result = parseLlmSuggestions(
      [
        "写作提示：",
        "1) 优化开头：精炼导语，突出矛盾或冲突。",
        "2) 案例补强：补充一个真实案例支撑论点。",
        "自动搜索：",
        "1) 行业案例 数据 支撑",
      ].join("\n"),
    );

    expect(result).not.toBeNull();
    expect(result?.prompts[0]).toMatchObject({
      title: "优化开头",
      priority: 1,
    });
    expect(result?.searches[0]).toMatchObject({
      query: "行业案例 数据 支撑",
      priority: 1,
    });
  });

  it("builds writing prompts that request the flow_write tool", () => {
    const prompt = buildWriteActionPrompt(
      {
        id: "polish",
        title: "润色表达",
        description: "收紧冗余",
        instruction: "请润色下面的草稿。",
      },
      "  原始草稿  ",
    );

    expect(prompt).toContain("flow_write");
    expect(prompt).toContain("polish");
    expect(prompt).toContain("原始草稿");
  });
});
