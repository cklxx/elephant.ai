import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { cancelTask, getTaskStatus } from "./api";

function clearDocumentCookies(): void {
  const cookies = document.cookie ? document.cookie.split(";") : [];
  for (const entry of cookies) {
    const name = entry.split("=")[0]?.trim();
    if (!name) {
      continue;
    }
    document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
  }
}

function mockNoContentResponse(): Response {
  return {
    ok: true,
    status: 204,
    statusText: "No Content",
    headers: new Headers(),
    json: vi.fn(),
    text: vi.fn().mockResolvedValue(""),
  } as unknown as Response;
}

function mockJsonResponse(payload: unknown): Response {
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    headers: new Headers({
      "content-type": "application/json",
    }),
    json: vi.fn().mockResolvedValue(payload),
    text: vi.fn().mockResolvedValue(JSON.stringify(payload)),
  } as unknown as Response;
}

describe("api csrf headers", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    document.head.innerHTML = "";
    clearDocumentCookies();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    document.head.innerHTML = "";
    clearDocumentCookies();
  });

  it("adds X-CSRF-Token from cookie for non-GET requests", async () => {
    document.cookie = "csrf_token=cookie-token; path=/";

    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(mockNoContentResponse());

    await cancelTask("task-123");

    const requestInit = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
    const headers = new Headers(requestInit?.headers);
    expect(headers.get("X-CSRF-Token")).toBe("cookie-token");
  });

  it("falls back to csrf meta tag for non-GET requests", async () => {
    const meta = document.createElement("meta");
    meta.setAttribute("name", "csrf-token");
    meta.setAttribute("content", "meta-token");
    document.head.appendChild(meta);

    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(mockNoContentResponse());

    await cancelTask("task-456");

    const requestInit = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
    const headers = new Headers(requestInit?.headers);
    expect(headers.get("X-CSRF-Token")).toBe("meta-token");
  });

  it("does not add csrf header for GET requests", async () => {
    document.cookie = "csrf_token=cookie-token; path=/";

    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        mockJsonResponse({
          run_id: "task-789",
          session_id: "session-1",
          status: "running",
          created_at: "2026-02-23T00:00:00.000Z",
        }),
      );

    await getTaskStatus("task-789");

    const requestInit = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
    const headers = new Headers(requestInit?.headers);
    expect(headers.has("X-CSRF-Token")).toBe(false);
  });
});
