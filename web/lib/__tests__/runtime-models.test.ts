import { describe, expect, it, vi } from "vitest";
import { getRuntimeModelCatalog } from "@/lib/api";

vi.mock("@/lib/api-base", () => ({ buildApiUrl: (path: string) => path }));

global.fetch = vi.fn().mockResolvedValue({
  ok: true,
  status: 200,
  headers: new Headers(),
  json: async () => ({ providers: [] }),
}) as any;

describe("getRuntimeModelCatalog", () => {
  it("fetches the runtime model catalog", async () => {
    const result = await getRuntimeModelCatalog();
    expect(result.providers).toEqual([]);
    expect(fetch).toHaveBeenCalledWith(
      "/api/internal/config/runtime/models",
      expect.objectContaining({ credentials: "include" }),
    );
  });
});
