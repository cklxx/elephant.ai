import { AttachmentPayload } from "@/lib/types";
import { humanizeToolName } from "@/lib/utils";

export function stripSystemReminders(content: string): string {
  if (!content) return "";
  if (!content.includes("<system-reminder>")) {
    return content.trim();
  }

  const lines = content.split("\n");
  const filtered: string[] = [];
  let inReminder = false;

  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith("<system-reminder>")) {
      inReminder = true;
      if (trimmed.endsWith("</system-reminder>")) {
        inReminder = false;
      }
      continue;
    }
    if (trimmed.endsWith("</system-reminder>")) {
      inReminder = false;
      continue;
    }
    if (!inReminder) {
      filtered.push(line);
    }
  }

  return filtered.join("\n").trim();
}

function getAttachmentNames(
  attachments?: Record<string, AttachmentPayload> | null,
): string[] {
  if (!attachments) return [];
  return Object.keys(attachments).filter(
    (name) => typeof name === "string" && name.trim().length > 0,
  );
}

function summarizeAttachmentNames(
  attachments?: Record<string, AttachmentPayload> | null,
): string | undefined {
  const names = getAttachmentNames(attachments);
  if (names.length === 0) return undefined;
  if (names.length === 1) return names[0];
  if (names.length === 2) return `${names[0]}、${names[1]}`;
  return `${names[0]}、${names[1]} 等 ${names.length} 个`;
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function getTodoCounts(metadata?: Record<string, any> | null): {
  total?: number;
  inProgress?: number;
  pending?: number;
  completed?: number;
} | null {
  if (!metadata || typeof metadata !== "object") return null;

  const total = metadata.total_count;
  const inProgress = metadata.in_progress_count;
  const pending = metadata.pending_count;
  const completed = metadata.completed_count;

  if (
    isFiniteNumber(total) ||
    isFiniteNumber(inProgress) ||
    isFiniteNumber(pending) ||
    isFiniteNumber(completed)
  ) {
    return {
      total: isFiniteNumber(total) ? total : undefined,
      inProgress: isFiniteNumber(inProgress) ? inProgress : undefined,
      pending: isFiniteNumber(pending) ? pending : undefined,
      completed: isFiniteNumber(completed) ? completed : undefined,
    };
  }

  return null;
}

const TOOL_TITLE_HINT_MAX_LEN = 140;
const TOOL_TITLE_MAX_LEN = 56;

function normalizeOneLine(value: unknown): string | undefined {
  if (typeof value !== "string") return undefined;
  const trimmed = value.trim().replace(/\s+/g, " ");
  return trimmed.length > 0 ? trimmed : undefined;
}

function truncateWithEllipsis(value: string, maxLen: number): string {
  if (value.length <= maxLen) return value;
  if (maxLen <= 1) return "…".slice(0, maxLen);
  return `${value.slice(0, maxLen - 1)}…`;
}

function truncateHint(value: string, maxLen: number = TOOL_TITLE_HINT_MAX_LEN): string {
  return truncateWithEllipsis(value, maxLen);
}

function truncateToolTitle(base: string, hint?: string): string {
  if (!hint) return truncateWithEllipsis(base, TOOL_TITLE_MAX_LEN);
  const separator = "：";
  const full = `${base}${separator}${hint}`;
  if (full.length <= TOOL_TITLE_MAX_LEN) return full;
  if (base.length >= TOOL_TITLE_MAX_LEN) {
    return truncateWithEllipsis(base, TOOL_TITLE_MAX_LEN);
  }
  const availableHintLen = TOOL_TITLE_MAX_LEN - base.length - separator.length;
  if (availableHintLen <= 0) {
    return truncateWithEllipsis(base, TOOL_TITLE_MAX_LEN);
  }
  return `${base}${separator}${truncateWithEllipsis(hint, availableHintLen)}`;
}

function summarizeNameList(names: string[]): string | undefined {
  const cleaned = names
    .map((name) => normalizeOneLine(name))
    .filter((name): name is string => Boolean(name));
  if (cleaned.length === 0) return undefined;
  if (cleaned.length === 1) return truncateHint(cleaned[0]);
  if (cleaned.length === 2) return truncateHint(`${cleaned[0]}、${cleaned[1]}`);
  return truncateHint(`${cleaned[0]}、${cleaned[1]} 等 ${cleaned.length} 个`);
}

function formatHintValue(value: unknown): string | undefined {
  if (typeof value === "string") {
    const normalized = normalizeOneLine(value);
    return normalized ? truncateHint(normalized) : undefined;
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  if (typeof value === "boolean") {
    return value ? "true" : "false";
  }
  if (Array.isArray(value)) {
    const strings = value.filter((item) => typeof item === "string") as string[];
    return strings.length > 0 ? summarizeNameList(strings) : undefined;
  }
  return undefined;
}

function joinHints(parts: Array<string | undefined>, separator: string = " · "): string | undefined {
  const filtered = parts.filter((part): part is string => Boolean(part));
  if (filtered.length === 0) return undefined;
  return truncateHint(filtered.join(separator));
}

function pickFirstHint(
  args: Record<string, any> | null | undefined,
  keys: string[],
): string | undefined {
  if (!args || typeof args !== "object") return undefined;
  for (const key of keys) {
    if (!(key in args)) continue;
    const formatted = formatHintValue(args[key]);
    if (formatted) return formatted;
  }
  return undefined;
}

function pickMetadataUrl(metadata: Record<string, any> | null | undefined): string | undefined {
  if (!metadata || typeof metadata !== "object") return undefined;
  const urlValue =
    typeof metadata.url === "string"
      ? metadata.url
      : typeof metadata.browser?.url === "string"
        ? metadata.browser.url
        : undefined;
  return formatHintValue(urlValue);
}

function formatSandboxAction(action: Record<string, any>): string | undefined {
  const rawType =
    typeof action.action_type === "string" ? action.action_type : "";
  const type = rawType.trim().toUpperCase();
  if (!type) return undefined;

  switch (type) {
    case "CLICK":
      return "Click";
    case "DOUBLE_CLICK":
      return "Double click";
    case "RIGHT_CLICK":
      return "Right click";
    case "MOVE_TO":
      return "Move";
    case "SCROLL": {
      const dy =
        typeof action.dy === "number" && Number.isFinite(action.dy)
          ? action.dy
          : null;
      if (dy === null) return "Scroll";
      return dy < 0 ? "Scroll up" : "Scroll down";
    }
    case "TYPING": {
      const text = formatHintValue(action.text);
      return text ? `Type "${text}"` : "Type";
    }
    case "PRESS": {
      const key = formatHintValue(action.key);
      return key ? `Press ${key}` : "Press key";
    }
    case "HOTKEY": {
      const keys = Array.isArray(action.keys)
        ? action.keys.filter((item) => typeof item === "string")
        : [];
      return keys.length > 0 ? `Hotkey ${keys.join("+")}` : "Hotkey";
    }
    case "WAIT":
      return "Wait";
    default:
      return type.replace(/_/g, " ").toLowerCase();
  }
}

function formatSandboxDOMStep(step: Record<string, any>): string | undefined {
  const rawAction =
    typeof step.action === "string"
      ? step.action
      : typeof step.action_type === "string"
        ? step.action_type
        : "";
  const action = rawAction.trim().toLowerCase();
  if (!action) return undefined;

  const selector = formatHintValue(step.selector);
  const url = formatHintValue(step.url);
  const text = formatHintValue(step.text ?? step.value);
  const key = formatHintValue(step.key);
  const attribute = formatHintValue(step.attribute);

  switch (action) {
    case "goto":
    case "navigate":
      return url ? `Open ${url}` : "Open page";
    case "click":
      return selector ? `Click ${selector}` : "Click";
    case "hover":
      return selector ? `Hover ${selector}` : "Hover";
    case "focus":
      return selector ? `Focus ${selector}` : "Focus";
    case "fill":
      if (selector && text) return `Fill ${selector}`;
      return selector ? `Fill ${selector}` : "Fill";
    case "type":
      return selector ? `Type ${selector}` : "Type";
    case "press":
      return key ? `Press ${key}` : "Press key";
    case "select":
      return selector ? `Select ${selector}` : "Select";
    case "wait_for":
      return selector ? `Wait for ${selector}` : "Wait";
    case "get_text":
      return selector ? `Read ${selector}` : "Read text";
    case "get_html":
      return selector ? `Read HTML ${selector}` : "Read HTML";
    case "get_attribute":
      if (selector && attribute) return `Read ${attribute} ${selector}`;
      return selector ? `Read ${selector}` : "Read attribute";
    case "evaluate":
      return "Evaluate script";
    default:
      return action.replace(/_/g, " ");
  }
}

function summarizeSandboxDOMSteps(steps: any): string | undefined {
  if (!Array.isArray(steps) || steps.length === 0) return undefined;
  const labels = steps
    .map((step) =>
      step && typeof step === "object"
        ? formatSandboxDOMStep(step as Record<string, any>)
        : undefined,
    )
    .filter((label): label is string => Boolean(label));

  if (labels.length === 0) return undefined;
  if (labels.length <= 3) return labels.join(" · ");
  return `${labels[0]} · ${labels[1]} 等 ${labels.length} 步`;
}

function summarizeSandboxActions(actions: any): string | undefined {
  if (!Array.isArray(actions) || actions.length === 0) return undefined;
  const labels = actions
    .map((action) =>
      action && typeof action === "object"
        ? formatSandboxAction(action as Record<string, any>)
        : undefined,
    )
    .filter((label): label is string => Boolean(label));

  if (labels.length === 0) return undefined;
  if (labels.length <= 3) return labels.join(" · ");
  return `${labels[0]} · ${labels[1]} 等 ${labels.length} 步`;
}

const FALLBACK_HINT_KEYS = [
  "url",
  "path",
  "file_path",
  "filePath",
  "uri",
  "code_path",
  "command",
  "pattern",
  "query",
  "q",
  "prompt",
  "message",
  "input",
  "name",
  "id",
];

export function userFacingToolTitle(input: {
  toolName: string;
  arguments?: Record<string, any> | null;
  metadata?: Record<string, any> | null;
  attachments?: Record<string, AttachmentPayload> | null;
}): string {
  const tool = input.toolName.toLowerCase().trim();
  const base = humanizeToolName(tool);
  const formatTitle = (hint?: string) => truncateToolTitle(base, hint);

  if (!tool) {
    return formatTitle();
  }

  if (tool === "web_search" || tool === "search_web" || tool === "websearch") {
    const query = pickFirstHint(input.arguments, ["query", "q"]);
    return formatTitle(query);
  }

  // Playwright MCP browser tools — show the primary argument as title hint
  if (tool.startsWith("mcp__playwright__browser_")) {
    const url = pickFirstHint(input.arguments, ["url"]);
    const text = pickFirstHint(input.arguments, ["text", "value"]);
    const ref = pickFirstHint(input.arguments, ["ref", "element"]);
    const hint = joinHints([url, text, ref]);
    return formatTitle(hint);
  }

  if (tool === "web_fetch" || tool === "read_url_content" || tool === "open_browser_url" || tool === "read_browser_page") {
    const url = pickFirstHint(input.arguments, ["url"]) ?? pickMetadataUrl(input.metadata);
    return formatTitle(url);
  }

  if (tool === "bash" || tool === "run_command") {
    const command = pickFirstHint(input.arguments, ["command"]);
    return formatTitle(command);
  }

  if (tool === "code_execute" || tool === "python_execute") {
    const language = pickFirstHint(input.arguments, ["language"]);
    const codePath = pickFirstHint(input.arguments, ["code_path", "path"]);
    const hint = joinHints([language, codePath]);
    return formatTitle(hint);
  }

  if (tool === "file_read" || tool === "read_resource") {
    const path = pickFirstHint(input.arguments, ["path", "uri"]);
    return formatTitle(path);
  }

  if (tool === "file_write" || tool === "write_to_file") {
    const path = pickFirstHint(input.arguments, ["path"]);
    return formatTitle(path);
  }

  if (tool === "file_edit" || tool === "replace_file_content") {
    const path = pickFirstHint(input.arguments, ["file_path", "path", "filePath"]);
    return formatTitle(path);
  }

  if (tool === "multi_replace_file_content") {
    const paths = formatHintValue(
      input.arguments?.paths ?? input.arguments?.file_paths ?? input.arguments?.filePaths,
    );
    return formatTitle(paths);
  }

  if (tool === "list_dir" || tool === "list_files") {
    const path = pickFirstHint(input.arguments, ["path"]);
    return formatTitle(path);
  }

  if (tool === "grep" || tool === "ripgrep" || tool === "grep_search" || tool === "search_in_file") {
    const pattern = pickFirstHint(input.arguments, ["pattern", "query"]);
    const path = pickFirstHint(input.arguments, ["path"]);
    const hint = joinHints([pattern, path]);
    return formatTitle(hint);
  }

  if (tool === "find" || tool === "find_by_name") {
    const name = pickFirstHint(input.arguments, ["name"]);
    const path = pickFirstHint(input.arguments, ["path"]);
    const hint = joinHints([name, path]);
    return formatTitle(hint);
  }

  if (tool === "click_browser_element") {
    const hint = pickFirstHint(input.arguments, ["text", "selector", "uid", "id"]);
    return formatTitle(hint);
  }

  if (tool === "type_browser_element") {
    const hint = pickFirstHint(input.arguments, ["text", "value", "uid", "selector"]);
    return formatTitle(hint);
  }

  if (tool === "scroll_browser_page") {
    const hint = pickFirstHint(input.arguments, ["direction", "amount"]);
    return formatTitle(hint);
  }

  if (tool === "notify_user") {
    const message = pickFirstHint(input.arguments, ["message"]);
    return formatTitle(message);
  }

  if (tool === "todo_update") {
    const counts = getTodoCounts(input.metadata);
    if (counts) {
      const parts: string[] = [];
      if (typeof counts.total === "number") parts.push(`共 ${counts.total} 项`);
      if (typeof counts.inProgress === "number") parts.push(`进行中 ${counts.inProgress}`);
      if (typeof counts.pending === "number") parts.push(`待办 ${counts.pending}`);
      if (typeof counts.completed === "number") parts.push(`已完成 ${counts.completed}`);
      const hint = truncateHint(parts.join(" / "));
      return formatTitle(hint);
    }
    const todos = input.arguments?.todos;
    if (Array.isArray(todos)) {
      return formatTitle(`${todos.length} 项`);
    }
    return formatTitle();
  }

  if (tool === "todo_read") {
    return formatTitle();
  }

  if (tool === "artifacts_write" || tool === "artifacts_list" || tool === "artifacts_delete") {
    const name = pickFirstHint(input.arguments, ["name"]);
    if (name) return formatTitle(name);
    const namesHint = formatHintValue(input.arguments?.names);
    if (namesHint) return formatTitle(namesHint);
    const attachmentNames = summarizeAttachmentNames(input.attachments);
    return formatTitle(attachmentNames);
  }

  if (tool === "text_to_image" || tool === "video_generate") {
    const prompt = pickFirstHint(input.arguments, ["prompt"]);
    return formatTitle(prompt);
  }

  if (tool === "vision_analyze") {
    const hint =
      pickFirstHint(input.arguments, ["url", "path", "file_path", "name"]) ??
      pickMetadataUrl(input.metadata);
    return formatTitle(hint);
  }

  const fallback = pickFirstHint(input.arguments, FALLBACK_HINT_KEYS) ?? pickMetadataUrl(input.metadata);
  return formatTitle(fallback);
}

export function userFacingToolSummary(input: {
  toolName: string;
  result?: string | null;
  error?: string | null;
  metadata?: Record<string, any> | null;
  attachments?: Record<string, AttachmentPayload> | null;
}): string | undefined {
  const tool = input.toolName.toLowerCase().trim();

  if (input.error && input.error.trim()) {
    return input.error.trim().length > 100
      ? `${input.error.trim().slice(0, 100)}…`
      : input.error.trim();
  }

  if (tool === "todo_update") {
    const counts = getTodoCounts(input.metadata);
    if (counts) {
      const parts: string[] = [];
      if (typeof counts.total === "number") parts.push(`共 ${counts.total} 项`);
      if (typeof counts.inProgress === "number")
        parts.push(`进行中 ${counts.inProgress}`);
      if (typeof counts.pending === "number")
        parts.push(`待办 ${counts.pending}`);
      if (typeof counts.completed === "number")
        parts.push(`已完成 ${counts.completed}`);
      if (parts.length > 0) {
        return `（${parts.join(" / ")}）`;
      }
    }
    return "";
  }

  if (tool === "artifacts_write") {
    const names = summarizeAttachmentNames(input.attachments);
    if (names) {
      return `已生成文件：${names}`;
    }

    const sanitized = stripSystemReminders(input.result ?? "");
    const match = sanitized.match(/^Saved\s+(.+?)\s+\((.+?)\)\s*$/i);
    if (match) {
      return `已生成文件：${match[1]}`;
    }
  }

  const sanitized = stripSystemReminders(input.result ?? "");
  if (!sanitized) return undefined;
  return sanitized.length > 100 ? `${sanitized.slice(0, 100)}…` : sanitized;
}

export function userFacingToolResultText(input: {
  toolName?: string | null;
  result?: string | null;
  metadata?: Record<string, any> | null;
  attachments?: Record<string, AttachmentPayload> | null;
}): string {
  const tool = (input.toolName ?? "").toLowerCase().trim();
  const sanitized = stripSystemReminders(input.result ?? "");

  if (tool === "artifacts_write") {
    const names = summarizeAttachmentNames(input.attachments);
    if (names) {
      return `已生成文件：${names}`;
    }
    const match = sanitized.match(/^Saved\s+(.+?)\s+\((.+?)\)\s*$/i);
    if (match) {
      return `已生成文件：${match[1]}`;
    }
  }

  if (tool === "todo_update") {
    // Keep the task list, but strip internal reminders.
    return sanitized;
  }

  return sanitized;
}
