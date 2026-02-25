export type HtmlValidationIssue = {
  level: "error" | "warning";
  message: string;
};

function decodeBase64ToText(value: string): string {
  const binary = atob(value);
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

export function decodeHtmlFromDataUri(uri: string): string | null {
  if (!uri.startsWith("data:")) return null;
  const commaIndex = uri.indexOf(",");
  if (commaIndex === -1) return null;
  const meta = uri.slice(5, commaIndex);
  const data = uri.slice(commaIndex + 1);
  if (/;base64/i.test(meta)) {
    try {
      return decodeBase64ToText(data);
    } catch (error) {
      console.error("Failed to decode base64 HTML", error);
      return null;
    }
  }
  try {
    return decodeURIComponent(data);
  } catch (error) {
    console.error("Failed to decode HTML data URI", error);
    return null;
  }
}

export async function loadHtmlSource(uri: string): Promise<string> {
  const decoded = decodeHtmlFromDataUri(uri);
  if (decoded !== null) return decoded;
  const response = await fetch(uri);
  if (!response.ok) {
    throw new Error(`Failed to load HTML (${response.status})`);
  }
  return response.text();
}

export function ensureViewportMeta(html: string): string {
  const trimmed = html.trim();
  if (!trimmed) return html;
  const lower = trimmed.toLowerCase();
  if (lower.includes('name="viewport"') || lower.includes("name='viewport'")) {
    return html;
  }

  const viewportTag =
    '<meta name="viewport" content="width=device-width, initial-scale=1" />';
  if (/<head\b[^>]*>/i.test(trimmed)) {
    return trimmed.replace(
      /<head\b[^>]*>/i,
      (match) => `${match}\n  ${viewportTag}`,
    );
  }
  if (/<body\b[^>]*>/i.test(trimmed)) {
    return trimmed.replace(
      /<body\b[^>]*>/i,
      (match) => `<head>\n  ${viewportTag}\n</head>\n${match}`,
    );
  }
  if (/<html\b[^>]*>/i.test(trimmed)) {
    return trimmed.replace(
      /<html\b[^>]*>/i,
      (match) => `${match}\n<head>\n  ${viewportTag}\n</head>`,
    );
  }

  return `<!doctype html><html><head>${viewportTag}</head><body>${trimmed}</body></html>`;
}

export function ensureBaseHref(html: string, baseHref: string | null): string {
  if (!baseHref) return html;
  const trimmed = html.trim();
  if (!trimmed) return html;
  if (/<base\b[^>]*>/i.test(trimmed)) {
    return html;
  }
  const baseTag = `<base href="${baseHref}">`;
  if (/<head\b[^>]*>/i.test(trimmed)) {
    return trimmed.replace(
      /<head\b[^>]*>/i,
      (match) => `${match}\n  ${baseTag}`,
    );
  }
  if (/<html\b[^>]*>/i.test(trimmed)) {
    return trimmed.replace(
      /<html\b[^>]*>/i,
      (match) => `${match}\n<head>\n  ${baseTag}\n</head>`,
    );
  }
  return `<head>${baseTag}</head>${trimmed}`;
}

function buildHtmlPreviewHtml(html: string, baseHref: string | null): string {
  const withViewport = ensureViewportMeta(html);
  return ensureBaseHref(withViewport, baseHref);
}

export function buildHtmlPreviewUrl(
  html: string,
  baseHref: string | null,
): { url: string; shouldRevoke: boolean } {
  const normalized = buildHtmlPreviewHtml(html, baseHref);
  if (typeof URL === "undefined" || typeof URL.createObjectURL !== "function") {
    return {
      url: `data:text/html;charset=utf-8,${encodeURIComponent(normalized)}`,
      shouldRevoke: false,
    };
  }
  const blob = new Blob([normalized], { type: "text/html" });
  return { url: URL.createObjectURL(blob), shouldRevoke: true };
}

export function validateHtmlSource(html: string): HtmlValidationIssue[] {
  const trimmed = html.trim();
  if (!trimmed) {
    return [{ level: "error", message: "HTML is empty." }];
  }

  const lower = trimmed.toLowerCase();
  const issues: HtmlValidationIssue[] = [];
  const htmlTags = (lower.match(/<html\b/g) ?? []).length;
  const bodyTags = (lower.match(/<body\b/g) ?? []).length;
  const headTags = (lower.match(/<head\b/g) ?? []).length;

  if (!lower.includes("<!doctype html")) {
    issues.push({ level: "warning", message: "Missing <!DOCTYPE html>." });
  }
  if (htmlTags === 0) {
    issues.push({ level: "warning", message: "Missing <html> tag." });
  } else if (htmlTags > 1) {
    issues.push({ level: "error", message: "Multiple <html> tags found." });
  }
  if (headTags === 0) {
    issues.push({ level: "warning", message: "Missing <head> tag." });
  } else if (headTags > 1) {
    issues.push({ level: "warning", message: "Multiple <head> tags found." });
  }
  if (bodyTags === 0) {
    issues.push({ level: "warning", message: "Missing <body> tag." });
  } else if (bodyTags > 1) {
    issues.push({ level: "warning", message: "Multiple <body> tags found." });
  }
  if (!lower.includes("<meta charset")) {
    issues.push({ level: "warning", message: "Missing <meta charset>." });
  }
  if (!/name=["']viewport["']/.test(lower)) {
    issues.push({
      level: "warning",
      message: 'Missing <meta name="viewport">.',
    });
  }
  if (!lower.includes("<title")) {
    issues.push({ level: "warning", message: "Missing <title> tag." });
  }

  const openScripts = (lower.match(/<script\b/g) ?? []).length;
  const closeScripts = (lower.match(/<\/script>/g) ?? []).length;
  if (openScripts !== closeScripts) {
    issues.push({ level: "error", message: "Mismatched <script> tags." });
  }

  const openStyles = (lower.match(/<style\b/g) ?? []).length;
  const closeStyles = (lower.match(/<\/style>/g) ?? []).length;
  if (openStyles !== closeStyles) {
    issues.push({ level: "warning", message: "Mismatched <style> tags." });
  }

  return issues;
}
