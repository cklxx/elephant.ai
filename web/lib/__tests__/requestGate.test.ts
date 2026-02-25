import { describe, expect, it } from "vitest";
import { createRequestGate } from "@/lib/requestGate";

describe("createRequestGate", () => {
  it("tracks only the latest token", () => {
    const gate = createRequestGate();
    const first = gate.next();

    expect(gate.isLatest(first)).toBe(true);

    const second = gate.next();
    expect(gate.isLatest(first)).toBe(false);
    expect(gate.isLatest(second)).toBe(true);
  });

  it("invalidates tokens on reset", () => {
    const gate = createRequestGate();
    const token = gate.next();

    gate.reset();

    expect(gate.isLatest(token)).toBe(false);
    const nextToken = gate.next();
    expect(gate.isLatest(nextToken)).toBe(true);
  });
});
