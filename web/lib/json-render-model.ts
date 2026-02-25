export type JsonRenderElement = {
  key?: string;
  type: string;
  props?: Record<string, any>;
  children?: Array<JsonRenderElement | string>;
};

export type JsonRenderTree = {
  root: JsonRenderElement | null;
  elements: Record<string, JsonRenderElement>;
};

export type JsonRenderPatch = {
  op: string;
  path: string;
  value?: any;
};

export function parseJsonRenderPayload(payload: string): JsonRenderTree {
  const trimmed = typeof payload === "string" ? payload.trim() : "";
  if (!trimmed) {
    throw new Error("Empty UI payload.");
  }

  try {
    const parsed = JSON.parse(trimmed);
    return normalizeJsonRenderValue(parsed);
  } catch {
    // Fall through to JSONL parsing.
  }

  const lines = trimmed.split(/\r?\n/);
  const patches: JsonRenderPatch[] = [];
  const entries: any[] = [];
  const errors: string[] = [];

  for (const line of lines) {
    const candidate = line.trim();
    if (!candidate) {
      continue;
    }
    try {
      const parsed = JSON.parse(candidate);
      if (isJsonRenderPatch(parsed)) {
        patches.push(parsed);
      } else {
        entries.push(parsed);
      }
    } catch (err) {
      errors.push(String(err));
    }
  }

  if (patches.length > 0) {
    return applyJsonRenderPatches({ root: null, elements: {} }, patches);
  }

  if (entries.length > 0) {
    return normalizeJsonRenderValue(entries);
  }

  if (errors.length > 0) {
    throw new Error("Unable to parse json-render payload.");
  }

  return { root: null, elements: {} };
}

export function normalizeJsonRenderValue(value: any): JsonRenderTree {
  const normalized = normalizeJsonRenderInput(value);
  if (normalized == null) {
    return { root: null, elements: {} };
  }

  if (isUiMessageBundle(normalized)) {
    const root = buildRootFromUiMessages(normalized.messages);
    const elements = indexJsonRenderElements(root);
    return { root, elements };
  }

  if (isJsonRenderTreeLike(normalized)) {
    const elements = normalizeElementsMap(normalized.elements);
    let root: JsonRenderElement | null = null;
    if (typeof normalized.root === "string") {
      root = elements[normalized.root] ?? null;
    } else if (isJsonRenderElement(normalized.root)) {
      root = normalizeElement(normalized.root);
    }
    if (root && Object.keys(elements).length === 0) {
      return { root, elements: indexJsonRenderElements(root) };
    }
    if (root && root.key && !elements[root.key]) {
      elements[root.key] = root;
    }
    return { root, elements };
  }

  if (Array.isArray(normalized)) {
    const root = {
      key: "root",
      type: "Column",
      props: { gap: 12 },
      children: normalized
        .map((entry, idx) => toJsonRenderElement(entry, `item-${idx}`))
        .filter(Boolean) as JsonRenderElement[],
    } satisfies JsonRenderElement;
    return { root, elements: indexJsonRenderElements(root) };
  }

  if (isJsonRenderElement(normalized)) {
    const root = normalizeElement(normalized);
    return { root, elements: indexJsonRenderElements(root) };
  }

  return { root: null, elements: {} };
}

function normalizeJsonRenderInput(value: any): any {
  if (value == null) {
    return value;
  }
  if (typeof value === "string") {
    const parsed = parseJsonValue(value);
    return parsed !== undefined ? normalizeJsonRenderInput(parsed) : value;
  }
  if (Array.isArray(value)) {
    return value.map((entry) => normalizeJsonRenderInput(entry));
  }
  if (!isPlainObject(value)) {
    return value;
  }

  const maybeWrapperType = typeof value.type === "string" ? value.type : "";
  const isWrapper =
    !maybeWrapperType ||
    maybeWrapperType === "json-render" ||
    maybeWrapperType === "json_render";

  if (isWrapper && typeof value.content === "string") {
    const parsed = parseJsonValue(value.content);
    if (parsed !== undefined) {
      return normalizeJsonRenderInput(parsed);
    }
  }
  if (isWrapper && typeof value.data === "string") {
    const parsed = parseJsonValue(value.data);
    if (parsed !== undefined) {
      return normalizeJsonRenderInput(parsed);
    }
  }
  if (isWrapper && value.content !== undefined) {
    return normalizeJsonRenderInput(value.content);
  }
  if (maybeWrapperType === "json-render" || maybeWrapperType === "json_render") {
    if (value.data !== undefined) {
      return normalizeJsonRenderInput(value.data);
    }
  }
  if (maybeWrapperType === "page" && value.body !== undefined) {
    return normalizeJsonRenderInput(value.body);
  }
  if (typeof value.view === "string" && !("type" in value)) {
    const { view, ...rest } = value;
    return { type: view, ...rest };
  }

  return value;
}

function parseJsonValue(value: string): any | undefined {
  try {
    return JSON.parse(value);
  } catch {
    return undefined;
  }
}

function isJsonRenderTreeLike(value: any): value is {
  root?: JsonRenderElement | string | null;
  elements?: Record<string, JsonRenderElement>;
} {
  return isPlainObject(value) && ("root" in value || "elements" in value);
}

function normalizeElementsMap(input: any): Record<string, JsonRenderElement> {
  if (!isPlainObject(input)) {
    return {};
  }
  const out: Record<string, JsonRenderElement> = {};
  for (const [key, element] of Object.entries(input)) {
    if (!isJsonRenderElement(element)) {
      continue;
    }
    out[key] = normalizeElement({ ...element, key: element.key ?? key });
  }
  return out;
}

function normalizeElement(element: JsonRenderElement): JsonRenderElement {
  const props = extractElementProps(element as Record<string, any>);
  const children = normalizeChildren(extractChildren(element as Record<string, any>));
  return {
    key: element.key,
    type: element.type,
    props,
    children,
  };
}

function indexJsonRenderElements(root: JsonRenderElement | null): Record<string, JsonRenderElement> {
  if (!root) {
    return {};
  }
  let counter = 0;
  const elements: Record<string, JsonRenderElement> = {};
  const visit = (node: JsonRenderElement) => {
    if (!node.key) {
      counter += 1;
      node.key = `node-${counter}`;
    }
    elements[node.key] = node;
    const children = Array.isArray(node.children) ? node.children : [];
    for (const child of children) {
      if (isJsonRenderElement(child)) {
        visit(child);
      }
    }
  };
  visit(root);
  return elements;
}

function isJsonRenderElement(value: any): value is JsonRenderElement {
  return (
    isPlainObject(value) &&
    typeof value.type === "string" &&
    (value.props === undefined || isPlainObject(value.props)) &&
    (value.children === undefined || Array.isArray(value.children))
  );
}

function isJsonRenderPatch(value: any): value is JsonRenderPatch {
  return (
    isPlainObject(value) &&
    typeof value.op === "string" &&
    typeof value.path === "string"
  );
}

function isUiMessageBundle(value: any): value is { type: string; messages: any[] } {
  return (
    isPlainObject(value) &&
    value.type === "ui" &&
    Array.isArray(value.messages)
  );
}

function buildRootFromUiMessages(messages: any[]): JsonRenderElement {
  const children = messages
    .map((message, idx) => toJsonRenderElement(message, `msg-${idx}`))
    .filter(Boolean) as JsonRenderElement[];
  return {
    key: "root",
    type: "Column",
    props: { gap: 12 },
    children,
  };
}

function toJsonRenderElement(value: any, fallbackKey: string): JsonRenderElement | null {
  if (!isPlainObject(value)) {
    return {
      key: fallbackKey,
      type: "Text",
      props: { text: String(value ?? "") },
    };
  }
  const rawType = typeof value.type === "string" ? value.type : "Text";
  const props = extractElementProps(value);
  const children = normalizeChildren(extractChildren(value));
  return {
    key: value.key ?? fallbackKey,
    type: rawType,
    props,
    children,
  };
}

function extractElementProps(value: Record<string, any>): Record<string, any> | undefined {
  const props: Record<string, any> = {};
  if (isPlainObject(value.props)) {
    Object.assign(props, value.props);
  }
  for (const [key, val] of Object.entries(value)) {
    if (key === "type" || key === "key" || key === "props" || key === "children") {
      continue;
    }
    if (key === "body") {
      continue;
    }
    props[key] = val;
  }
  return Object.keys(props).length > 0 ? props : undefined;
}

function extractChildren(value: Record<string, any>): any[] | undefined {
  if (Array.isArray(value.children)) {
    return value.children;
  }
  if (value.body !== undefined) {
    return [value.body];
  }
  return undefined;
}

function normalizeChildren(
  children: any[] | undefined,
): Array<JsonRenderElement | string> | undefined {
  if (!Array.isArray(children)) {
    return undefined;
  }
  const normalized: Array<JsonRenderElement | string> = [];
  children.forEach((child, idx) => {
    if (child == null) {
      return;
    }
    if (typeof child === "string") {
      normalized.push(child);
      return;
    }
    const element = toJsonRenderElement(child, `child-${idx}`);
    if (element) {
      normalized.push(element);
    }
  });
  return normalized;
}

function applyJsonRenderPatches(base: JsonRenderTree, patches: JsonRenderPatch[]): JsonRenderTree {
  const tree: JsonRenderTree = {
    root: base.root,
    elements: { ...base.elements },
  };

  for (const patch of patches) {
    applyPatch(tree, patch);
  }

  return normalizeJsonRenderValue({ root: tree.root, elements: tree.elements });
}

function applyPatch(tree: JsonRenderTree, patch: JsonRenderPatch) {
  const op = patch.op.toLowerCase();
  const segments = parsePointer(patch.path);
  if (segments.length === 0) {
    return;
  }

  if (op === "remove") {
    removePointerValue(tree as any, segments);
    return;
  }

  const value = patch.value;
  if (op === "add") {
    addPointerValue(tree as any, segments, value);
    return;
  }

  setPointerValue(tree as any, segments, value);
}

function parsePointer(path: string): string[] {
  if (!path) {
    return [];
  }
  const trimmed = path.startsWith("/") ? path.slice(1) : path;
  if (!trimmed) {
    return [];
  }
  return trimmed.split("/").map(decodePointerSegment);
}

function decodePointerSegment(segment: string): string {
  return segment.replace(/~1/g, "/").replace(/~0/g, "~");
}

function setPointerValue(target: any, segments: string[], value: any) {
  const lastIndex = segments.length - 1;
  let current = target;
  for (let i = 0; i < lastIndex; i += 1) {
    const segment = segments[i];
    const next = segments[i + 1];
    const nextIsIndex = isArrayIndex(next);
    if (!current[segment] || typeof current[segment] !== "object") {
      current[segment] = nextIsIndex ? [] : {};
    }
    current = current[segment];
  }
  const last = segments[lastIndex];
  if (Array.isArray(current) && isArrayIndex(last)) {
    current[Number(last)] = value;
    return;
  }
  current[last] = value;
}

function addPointerValue(target: any, segments: string[], value: any) {
  const lastIndex = segments.length - 1;
  let current = target;
  for (let i = 0; i < lastIndex; i += 1) {
    const segment = segments[i];
    const next = segments[i + 1];
    const nextIsIndex = isArrayIndex(next);
    if (!current[segment] || typeof current[segment] !== "object") {
      current[segment] = nextIsIndex ? [] : {};
    }
    current = current[segment];
  }
  const last = segments[lastIndex];
  if (Array.isArray(current)) {
    if (last === "-") {
      current.push(value);
      return;
    }
    if (isArrayIndex(last)) {
      current.splice(Number(last), 0, value);
      return;
    }
  }
  if (current[last] && Array.isArray(current[last])) {
    current[last].push(value);
    return;
  }
  current[last] = value;
}

function removePointerValue(target: any, segments: string[]) {
  const lastIndex = segments.length - 1;
  let current = target;
  for (let i = 0; i < lastIndex; i += 1) {
    const segment = segments[i];
    if (!current || typeof current !== "object") {
      return;
    }
    current = current[segment];
  }
  const last = segments[lastIndex];
  if (Array.isArray(current) && isArrayIndex(last)) {
    current.splice(Number(last), 1);
    return;
  }
  if (current && typeof current === "object") {
    delete current[last];
  }
}

function isArrayIndex(value: string | undefined): boolean {
  if (!value) {
    return false;
  }
  if (value === "-") {
    return true;
  }
  return /^\d+$/.test(value);
}

function isPlainObject(value: unknown): value is Record<string, any> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}
