import { NextRequest, NextResponse } from "next/server";

import { resolveApiBaseUrl } from "@/lib/api-base";
import {
  decodeBase64Text,
  decodeDataUri,
} from "@/lib/attachment-text";
import { renderJsonRenderHtml } from "@/lib/json-render-ssr";
import { parseUIPayload } from "@/lib/ui-payload";

const MAX_PAYLOAD_BYTES = 1_000_000;

type PreviewRequestBody = {
  payload?: string;
  data?: string;
  uri?: string;
};

export async function POST(req: NextRequest) {
  let body: PreviewRequestBody | null = null;
  try {
    body = (await req.json()) as PreviewRequestBody;
  } catch {
    return new NextResponse("Invalid JSON body.", { status: 400 });
  }

  const payload =
    typeof body?.payload === "string" && body.payload.trim()
      ? body.payload
      : null;
  if (payload) {
    return renderPayloadResponse(payload);
  }

  const data = typeof body?.data === "string" ? body.data.trim() : "";
  if (data) {
    return renderPayloadResponse(decodeBase64Text(data));
  }

  const uri = typeof body?.uri === "string" ? body.uri.trim() : "";
  if (!uri) {
    return new NextResponse("Missing UI payload.", { status: 400 });
  }

  const fetched = await loadPayloadFromUri(uri, req);
  if (!fetched) {
    return new NextResponse("Unable to load UI payload.", { status: 422 });
  }

  return renderPayloadResponse(fetched);
}

function htmlResponse(html: string) {
  return new NextResponse(html, {
    headers: {
      "content-type": "text/html; charset=utf-8",
      "cache-control": "no-store",
    },
  });
}

function renderPayloadResponse(payload: string) {
  const html = renderUiHtmlFromPayload(payload);
  if (!html) {
    return new NextResponse("Unable to render UI payload.", { status: 422 });
  }
  return htmlResponse(html);
}

async function loadPayloadFromUri(
  uri: string,
  req: NextRequest,
): Promise<string | null> {
  if (uri.startsWith("data:")) {
    const decoded = decodeDataUri(uri);
    return decoded?.trim() ? decoded : null;
  }

  let target: URL;
  try {
    if (uri.startsWith("/")) {
      target = new URL(uri, req.nextUrl.origin);
    } else {
      target = new URL(uri);
    }
  } catch {
    return null;
  }

  if (target.protocol !== "http:" && target.protocol !== "https:") {
    return null;
  }

  const allowedOrigins = new Set<string>();
  allowedOrigins.add(req.nextUrl.origin);
  const apiBase = resolveApiBaseUrl();
  try {
    allowedOrigins.add(new URL(apiBase).origin);
  } catch {
    // Ignore invalid api base urls.
  }

  if (!allowedOrigins.has(target.origin)) {
    return null;
  }

  const response = await fetch(target.toString(), { cache: "no-store" });
  if (!response.ok) {
    return null;
  }
  const buffer = await response.arrayBuffer();
  if (buffer.byteLength > MAX_PAYLOAD_BYTES) {
    return null;
  }
  return new TextDecoder("utf-8").decode(buffer);
}

function renderUiHtmlFromPayload(payload: string): string | null {
  const ui = parseUIPayload(payload);
  if (ui.kind === "json-render") {
    return renderJsonRenderHtml(ui.tree);
  }
  return null;
}
