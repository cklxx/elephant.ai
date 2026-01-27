type A2UIMessage = {
  surfaceUpdate?: {
    surfaceId?: string;
    components?: Array<{
      id?: string;
      weight?: number;
      component?: Record<string, any>;
    }>;
  };
  dataModelUpdate?: {
    surfaceId?: string;
    path?: string;
    contents?: any;
  };
  beginRendering?: {
    surfaceId?: string;
    catalogId?: string;
    root?: string;
    styles?: Record<string, any>;
  };
  deleteSurface?: {
    surfaceId?: string;
  };
};

type A2UIComponent = {
  id: string;
  type: string;
  props: Record<string, any>;
  weight?: number;
};

type A2UISurface = {
  id: string;
  components: Record<string, A2UIComponent>;
  rootId?: string;
  dataModel: Record<string, any>;
  catalogId?: string;
  styles?: Record<string, any>;
};

type A2UIState = {
  surfaces: Record<string, A2UISurface>;
  surfaceOrder: string[];
};

type RenderContext = {
  surface: A2UISurface;
  dataModel: Record<string, any>;
  basePath: string;
};

const DEFAULT_SURFACE_ID = "default";

export function renderA2UIHtml(payload: string): string {
  const messages = parseA2UIMessagePayload(payload);
  return renderA2UIHtmlFromMessages(messages);
}

export function renderA2UIHtmlFromMessages(messages: A2UIMessage[]): string {
  const state = buildState(messages);
  return renderState(state);
}

export function parseA2UIMessagePayload(payload: string): A2UIMessage[] {
  const trimmed = typeof payload === "string" ? payload.trim() : "";
  if (!trimmed) {
    return [];
  }

  try {
    const parsed = JSON.parse(trimmed);
    return normalizeA2UIMessages(parsed);
  } catch {
    // Fall through to JSONL parsing.
  }

  const messages: A2UIMessage[] = [];
  const lines = trimmed.split(/\r?\n/);
  const errors: string[] = [];
  for (const line of lines) {
    const candidate = line.trim();
    if (!candidate) {
      continue;
    }
    try {
      const parsed = JSON.parse(candidate);
      messages.push(...normalizeA2UIMessages(parsed));
    } catch (err) {
      errors.push(String(err));
    }
  }

  if (messages.length === 0 && errors.length > 0) {
    throw new Error("Unable to parse A2UI payload.");
  }
  return messages;
}

export function decodeBase64Text(encoded: string): string {
  const trimmed = encoded?.trim?.() ?? "";
  if (!trimmed) {
    return "";
  }
  const buffer = (globalThis as any).Buffer;
  if (buffer) {
    return buffer.from(trimmed, "base64").toString("utf-8");
  }
  if (typeof atob === "function") {
    const binary = atob(trimmed);
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
    return new TextDecoder("utf-8").decode(bytes);
  }
  return trimmed;
}

export function decodeDataUri(uri: string): string | null {
  const match = uri.match(/^data:([^,]*?),(.*)$/);
  if (!match) {
    return null;
  }
  const meta = match[1] || "";
  const data = match[2] || "";
  if (meta.includes(";base64")) {
    return decodeBase64Text(data);
  }
  try {
    return decodeURIComponent(data);
  } catch {
    return data;
  }
}

function normalizeA2UIMessages(value: any): A2UIMessage[] {
  if (Array.isArray(value)) {
    return value.filter(isPlainObject) as A2UIMessage[];
  }
  if (isPlainObject(value)) {
    return [value as A2UIMessage];
  }
  return [];
}

function buildState(messages: A2UIMessage[]): A2UIState {
  const surfaces: Record<string, A2UISurface> = {};
  const surfaceOrder: string[] = [];

  const ensureSurface = (surfaceId?: string): A2UISurface => {
    const id = (surfaceId || DEFAULT_SURFACE_ID).trim() || DEFAULT_SURFACE_ID;
    if (!surfaces[id]) {
      surfaces[id] = {
        id,
        components: {},
        dataModel: {},
      };
      surfaceOrder.push(id);
    }
    return surfaces[id];
  };

  for (const message of messages) {
    if (!message || typeof message !== "object") {
      continue;
    }
    if (message.deleteSurface?.surfaceId) {
      const id = message.deleteSurface.surfaceId;
      delete surfaces[id];
      const index = surfaceOrder.indexOf(id);
      if (index >= 0) {
        surfaceOrder.splice(index, 1);
      }
      continue;
    }
    if (message.surfaceUpdate) {
      const surface = ensureSurface(message.surfaceUpdate.surfaceId);
      const components = message.surfaceUpdate.components || [];
      components.forEach((entry) => {
        const parsed = parseComponentEntry(entry);
        if (parsed) {
          surface.components[parsed.id] = parsed;
        }
      });
    }
    if (message.dataModelUpdate) {
      const surface = ensureSurface(message.dataModelUpdate.surfaceId);
      applyDataModelUpdate(surface, message.dataModelUpdate);
    }
    if (message.beginRendering) {
      const surface = ensureSurface(message.beginRendering.surfaceId);
      surface.rootId = message.beginRendering.root || surface.rootId;
      surface.catalogId = message.beginRendering.catalogId || surface.catalogId;
      if (message.beginRendering.styles) {
        surface.styles = message.beginRendering.styles;
      }
    }
  }

  return { surfaces, surfaceOrder };
}

function parseComponentEntry(entry: any): A2UIComponent | null {
  if (!entry || typeof entry !== "object") {
    return null;
  }
  const id = typeof entry.id === "string" ? entry.id.trim() : "";
  if (!id) {
    return null;
  }
  const component = entry.component;
  if (!component || typeof component !== "object") {
    return null;
  }
  const typeKey = Object.keys(component)[0];
  if (!typeKey) {
    return null;
  }
  const props = isPlainObject(component[typeKey]) ? component[typeKey] : {};
  const weight = typeof entry.weight === "number" ? entry.weight : undefined;
  return { id, type: typeKey, props, weight };
}

function applyDataModelUpdate(
  surface: A2UISurface,
  update: NonNullable<A2UIMessage["dataModelUpdate"]>,
) {
  if (!surface) {
    return;
  }
  const decoded = decodeDataModelContents(update.contents);
  const path = typeof update.path === "string" ? update.path.trim() : "";
  if (!path || path === "/") {
    surface.dataModel = decoded;
    return;
  }
  setPathValue(surface.dataModel, path, decoded);
}

function decodeDataModelContents(contents: any): Record<string, any> {
  if (Array.isArray(contents)) {
    return decodeDataEntries(contents);
  }
  if (isPlainObject(contents)) {
    return contents;
  }
  return {};
}

function decodeDataEntries(entries: any[]): Record<string, any> {
  const result: Record<string, any> = {};
  entries.forEach((entry) => {
    if (!entry || typeof entry !== "object") {
      return;
    }
    const key = typeof entry.key === "string" ? entry.key.trim() : "";
    if (!key) {
      return;
    }
    result[key] = decodeDataValue(entry);
  });
  return result;
}

function decodeDataValue(entry: Record<string, any>): any {
  if ("valueString" in entry) {
    return entry.valueString;
  }
  if ("valueNumber" in entry) {
    return entry.valueNumber;
  }
  if ("valueBoolean" in entry) {
    return entry.valueBoolean;
  }
  if ("valueMap" in entry) {
    const mapValue = entry.valueMap;
    if (Array.isArray(mapValue)) {
      return decodeDataEntries(mapValue);
    }
  }
  if ("valueList" in entry) {
    return decodeDataList(entry.valueList);
  }
  if ("valueArray" in entry) {
    return decodeDataList(entry.valueArray);
  }
  if ("value" in entry) {
    return entry.value;
  }
  return null;
}

function decodeDataList(value: any): any[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.map((entry) => {
    if (isPlainObject(entry)) {
      if ("key" in entry) {
        return decodeDataValue(entry as Record<string, any>);
      }
      if ("value" in entry) {
        return (entry as Record<string, any>).value;
      }
    }
    return entry;
  });
}

function renderState(state: A2UIState): string {
  const root: string[] = [];
  root.push("<!doctype html><html><head><meta charset=\"utf-8\">");
  root.push(
    "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">",
  );
  root.push("<style>");
  root.push(defaultStyles);
  root.push("</style></head><body><div class=\"a2ui-root\">");

  if (state.surfaceOrder.length === 0) {
    root.push(renderFallback("No A2UI surfaces to render."));
  } else {
    const showSurfaceLabel = state.surfaceOrder.length > 1;
    state.surfaceOrder.forEach((surfaceId) => {
      const surface = state.surfaces[surfaceId];
      if (!surface) {
        return;
      }
      root.push("<div class=\"a2ui-surface\">");
      if (showSurfaceLabel) {
        root.push(
          `<div class="a2ui-surface-label">${escapeHtml(surfaceId)}</div>`,
        );
      }
      if (surface.rootId) {
        root.push(
          renderComponentById(surface, surface.rootId, {
            surface,
            dataModel: surface.dataModel,
            basePath: "/",
          }),
        );
      } else {
        root.push(renderFallback("Surface missing root component."));
      }
      root.push("</div>");
    });
  }

  root.push("</div></body></html>");
  return root.join("");
}

function renderComponentById(
  surface: A2UISurface,
  componentId: string,
  context: RenderContext,
): string {
  const component = surface.components[componentId];
  if (!component) {
    return renderFallback(`Missing component: ${componentId}`);
  }
  return renderComponent(surface, component, context);
}

function renderComponent(
  surface: A2UISurface,
  component: A2UIComponent,
  context: RenderContext,
): string {
  const props = component.props || {};
  switch (component.type) {
    case "Column": {
      const children = renderChildren(surface, props.children, context, component.id);
      const alignment = flexAlignment(props.alignment, "stretch");
      const distribution = flexDistribution(props.distribution, "flex-start");
      return `<div class="a2ui-column" style="${alignment}${distribution}">${children}</div>`;
    }
    case "Row": {
      const children = renderChildren(surface, props.children, context, component.id);
      const alignment = flexAlignment(props.alignment, "flex-start");
      const distribution = flexDistribution(props.distribution, "flex-start");
      return `<div class="a2ui-row" style="${alignment}${distribution}">${children}</div>`;
    }
    case "List": {
      const children = renderChildren(surface, props.children, context, component.id);
      const direction = props.direction === "horizontal" ? "row" : "column";
      const alignment = flexAlignment(props.alignment, "stretch");
      return `<div class="a2ui-list" style="flex-direction:${direction};${alignment}">${children}</div>`;
    }
    case "Card": {
      const childId = typeof props.child === "string" ? props.child : "";
      if (!childId) {
        return renderFallback("Card missing child.");
      }
      return `<div class="a2ui-card">${renderComponentById(surface, childId, context)}</div>`;
    }
    case "Text": {
      const value = resolveBoundValue(props.text, context);
      const usageHint = typeof props.usageHint === "string" ? props.usageHint : "body";
      const text = escapeHtml(String(value ?? ""));
      if (usageHint === "h1") {
        return `<h1 class="a2ui-text a2ui-text-h1">${text}</h1>`;
      }
      if (usageHint === "h2") {
        return `<h2 class="a2ui-text a2ui-text-h2">${text}</h2>`;
      }
      if (usageHint === "h3") {
        return `<h3 class="a2ui-text a2ui-text-h3">${text}</h3>`;
      }
      if (usageHint === "h4") {
        return `<h4 class="a2ui-text a2ui-text-h4">${text}</h4>`;
      }
      if (usageHint === "h5") {
        return `<h5 class="a2ui-text a2ui-text-h5">${text}</h5>`;
      }
      if (usageHint === "caption") {
        return `<p class="a2ui-text a2ui-text-caption">${text}</p>`;
      }
      return `<p class="a2ui-text a2ui-text-body">${text}</p>`;
    }
    case "Image": {
      const url = resolveBoundValue(props.url, context);
      if (!url) {
        return renderFallback("Image missing url.");
      }
      const usageHint = typeof props.usageHint === "string" ? props.usageHint : "";
      const hintClass = imageClass(usageHint);
      const fitClass = objectFitClass(String(props.fit ?? ""));
      return `<img class="a2ui-image ${hintClass} ${fitClass}" src="${escapeAttribute(
        String(url),
      )}" alt="">`;
    }
    case "Video": {
      const url = resolveBoundValue(props.url, context);
      if (!url) {
        return renderFallback("Video missing url.");
      }
      return `<video class="a2ui-video" src="${escapeAttribute(
        String(url),
      )}" controls></video>`;
    }
    case "AudioPlayer": {
      const url = resolveBoundValue(props.url, context);
      const description = resolveBoundValue(props.description, context);
      const label = description ? escapeHtml(String(description)) : "";
      const parts = ["<div class=\"a2ui-audio\">"];
      if (label) {
        parts.push(`<div class="a2ui-audio-label">${label}</div>`);
      }
      if (url) {
        parts.push(
          `<audio class="a2ui-audio-player" src="${escapeAttribute(
            String(url),
          )}" controls></audio>`,
        );
      } else {
        parts.push(renderFallback("Audio missing url."));
      }
      parts.push("</div>");
      return parts.join("");
    }
    case "Button": {
      const childId = typeof props.child === "string" ? props.child : "";
      const actionName =
        typeof props.action?.name === "string" ? props.action.name : "";
      const isPrimary = Boolean(props.primary);
      const label = childId
        ? renderComponentById(surface, childId, context)
        : escapeHtml("Button");
      const className = isPrimary
        ? "a2ui-button a2ui-button-primary"
        : "a2ui-button";
      const parts = [
        `<div class="a2ui-button-row"><button class="${className}" type="button">${label}</button>`,
      ];
      if (actionName) {
        parts.push(`<span class="a2ui-badge">${escapeHtml(actionName)}</span>`);
      }
      parts.push("</div>");
      return parts.join("");
    }
    case "CheckBox": {
      const label = resolveBoundValue(props.label, context);
      const value = resolveBoundValue(props.value, context);
      const checked = Boolean(value);
      return `<label class="a2ui-checkbox"><input type="checkbox" ${
        checked ? "checked" : ""
      } disabled><span>${escapeHtml(String(label ?? ""))}</span></label>`;
    }
    case "TextField": {
      const label = resolveBoundValue(props.label, context);
      const value = resolveBoundValue(props.text, context);
      const fieldType =
        typeof props.textFieldType === "string" ? props.textFieldType : "shortText";
      const inputType =
        fieldType === "obscured"
          ? "password"
          : fieldType === "number"
            ? "number"
            : fieldType === "date"
              ? "date"
              : "text";
      const parts = ["<div class=\"a2ui-field\">"];
      if (label) {
        parts.push(
          `<label class="a2ui-field-label">${escapeHtml(
            String(label),
          )}</label>`,
        );
      }
      if (fieldType === "longText") {
        parts.push(
          `<textarea class="a2ui-textarea" readonly>${escapeHtml(
            String(value ?? ""),
          )}</textarea>`,
        );
      } else {
        parts.push(
          `<input class="a2ui-input" type="${inputType}" value="${escapeAttribute(
            String(value ?? ""),
          )}" readonly>`,
        );
      }
      parts.push("</div>");
      return parts.join("");
    }
    case "DateTimeInput": {
      const value = resolveBoundValue(props.value, context);
      const enableDate = props.enableDate !== false;
      const enableTime = props.enableTime !== false;
      const inputType = enableDate && enableTime ? "datetime-local" : enableTime ? "time" : "date";
      return `<input class="a2ui-input" type="${inputType}" value="${escapeAttribute(
        String(value ?? ""),
      )}" readonly>`;
    }
    case "Slider": {
      const value = resolveBoundValue(props.value, context);
      const minValue = typeof props.minValue === "number" ? props.minValue : 0;
      const maxValue = typeof props.maxValue === "number" ? props.maxValue : 100;
      const currentValue = typeof value === "number" ? value : 0;
      return `<div class="a2ui-slider"><input type="range" min="${minValue}" max="${maxValue}" value="${currentValue}" disabled><div class="a2ui-slider-value">Value: ${currentValue}</div></div>`;
    }
    case "MultipleChoice": {
      const selections = resolveBoundValue(props.selections, context);
      const selectionList = Array.isArray(selections)
        ? selections
        : selections
          ? [selections]
          : [];
      const options = Array.isArray(props.options) ? props.options : [];
      const maxAllowed =
        typeof props.maxAllowedSelections === "number"
          ? props.maxAllowedSelections
          : undefined;
      const inputType = maxAllowed === 1 ? "radio" : "checkbox";
      const parts = ["<div class=\"a2ui-multiple-choice\">"];
      options.forEach((option: any) => {
        const label = resolveBoundValue(option.label, context);
        const optionValue = option.value;
        const checked = selectionList.includes(optionValue);
        const text = label ?? optionValue ?? "";
        parts.push(
          `<label class="a2ui-option"><input type="${inputType}" ${
            checked ? "checked" : ""
          } disabled><span>${escapeHtml(String(text))}</span></label>`,
        );
      });
      parts.push("</div>");
      return parts.join("");
    }
    case "Tabs": {
      const tabItems = Array.isArray(props.tabItems) ? props.tabItems : [];
      if (tabItems.length === 0) {
        return renderFallback("Tabs missing items.");
      }
      const parts = ["<div class=\"a2ui-tabs\">"];
      tabItems.forEach((item: any, idx: number) => {
        const title =
          resolveBoundValue(item.title, context) ?? `Tab ${idx + 1}`;
        const childId = typeof item.child === "string" ? item.child : "";
        parts.push("<div class=\"a2ui-tab\">");
        parts.push(
          `<div class="a2ui-tab-title">${escapeHtml(String(title))}</div>`,
        );
        if (childId) {
          parts.push(renderComponentById(surface, childId, context));
        } else {
          parts.push(renderFallback("Tab missing child."));
        }
        parts.push("</div>");
      });
      parts.push("</div>");
      return parts.join("");
    }
    case "Modal": {
      const entryId = typeof props.entryPointChild === "string" ? props.entryPointChild : "";
      const contentId = typeof props.contentChild === "string" ? props.contentChild : "";
      if (!entryId || !contentId) {
        return renderFallback("Modal missing entryPointChild or contentChild.");
      }
      return `<div class="a2ui-modal"><div class="a2ui-modal-entry">${renderComponentById(
        surface,
        entryId,
        context,
      )}</div><div class="a2ui-modal-content">${renderComponentById(
        surface,
        contentId,
        context,
      )}</div></div>`;
    }
    case "Divider": {
      const axis = props.axis === "vertical" ? "vertical" : "horizontal";
      return axis === "vertical"
        ? "<div class=\"a2ui-divider-vertical\"></div>"
        : "<hr class=\"a2ui-divider\">";
    }
    case "Icon": {
      const name = resolveBoundValue(props.name, context);
      const label = name ? String(name) : "icon";
      return `<span class="a2ui-badge">${escapeHtml(label)}</span>`;
    }
    default:
      return renderFallback(`Unsupported component: ${component.type}`);
  }
}

function renderChildren(
  surface: A2UISurface,
  childrenDef: any,
  context: RenderContext,
  parentKey: string,
): string {
  if (!childrenDef) {
    return "";
  }

  if (Array.isArray(childrenDef)) {
    return childrenDef
      .map((childId, idx) =>
        renderWeightedChild(surface, childId, context, `${parentKey}-${idx}`),
      )
      .join("");
  }

  if (childrenDef.explicitList && Array.isArray(childrenDef.explicitList)) {
    return childrenDef.explicitList
      .map((childId: string, idx: number) =>
        renderWeightedChild(surface, childId, context, `${parentKey}-${idx}`),
      )
      .join("");
  }

  if (childrenDef.template && childrenDef.template.componentId) {
    const bindingPath = resolveBindingPath(childrenDef.template.dataBinding, context.basePath);
    const data = getPathValue(context.dataModel, bindingPath);
    if (data && typeof data === "object") {
      const entries = Array.isArray(data) ? data.map((_, idx) => String(idx)) : Object.keys(data);
      return entries
        .map((key) => {
          const nextContext = {
            ...context,
            basePath: appendPath(bindingPath, key),
          };
          return renderWeightedChild(
            surface,
            childrenDef.template.componentId,
            nextContext,
            `${parentKey}-template-${key}`,
          );
        })
        .join("");
    }
    return renderFallback("Template data binding empty.");
  }

  return "";
}

function renderWeightedChild(
  surface: A2UISurface,
  childId: string,
  context: RenderContext,
  key: string,
): string {
  if (typeof childId !== "string") {
    return renderFallback("Invalid child reference.");
  }
  const component = surface.components[childId];
  const weight = component?.weight;
  const content = renderComponentById(surface, childId, context);
  if (typeof weight === "number") {
    return `<div class="a2ui-weighted" style="flex-grow:${weight};">${content}</div>`;
  }
  return `<div class="a2ui-weighted">${content}</div>`;
}

function resolveBoundValue(value: any, context: RenderContext) {
  if (!isPlainObject(value)) {
    return value;
  }
  const { literalFound, literalValue, path } = extractBoundValue(value);
  if (!path && !literalFound) {
    return value;
  }

  if (path) {
    const absolutePath = resolveBindingPath(path, context.basePath);
    if (literalFound) {
      setPathValue(context.dataModel, absolutePath, literalValue);
    }
    const resolved = getPathValue(context.dataModel, absolutePath);
    if (resolved !== undefined) {
      return resolved;
    }
    return literalFound ? literalValue : undefined;
  }

  return literalValue;
}

function extractBoundValue(value: Record<string, any>): {
  literalFound: boolean;
  literalValue: any;
  path?: string;
} {
  let literalFound = false;
  let literalValue: any = undefined;

  if ("literalString" in value) {
    literalFound = true;
    literalValue = value.literalString;
  } else if ("literalNumber" in value) {
    literalFound = true;
    literalValue = value.literalNumber;
  } else if ("literalBoolean" in value) {
    literalFound = true;
    literalValue = value.literalBoolean;
  } else if ("literalArray" in value) {
    literalFound = true;
    literalValue = value.literalArray;
  } else if ("literalObject" in value) {
    literalFound = true;
    literalValue = value.literalObject;
  } else if ("literalMap" in value) {
    literalFound = true;
    literalValue = value.literalMap;
  } else if ("literalNull" in value) {
    literalFound = true;
    literalValue = null;
  }

  const path = typeof value.path === "string" ? value.path.trim() : undefined;
  return { literalFound, literalValue, path };
}

function resolveBindingPath(path: string | undefined, basePath: string): string {
  const trimmed = typeof path === "string" ? path.trim() : "";
  if (!trimmed) {
    return basePath || "/";
  }
  if (trimmed.startsWith("/")) {
    return trimmed;
  }
  if (!basePath || basePath === "/") {
    return `/${trimmed}`;
  }
  return `${basePath.replace(/\/$/, "")}/${trimmed}`;
}

function appendPath(basePath: string, suffix: string): string {
  if (!basePath || basePath === "/") {
    return `/${suffix}`;
  }
  return `${basePath.replace(/\/$/, "")}/${suffix}`;
}

function splitPath(path: string): string[] {
  return path
    .split("/")
    .map((segment) => segment.trim())
    .filter(Boolean);
}

function getPathValue(model: Record<string, any>, path: string): any {
  if (!model || !path) {
    return undefined;
  }
  const segments = splitPath(path);
  let current: any = model;
  for (const segment of segments) {
    if (!current || typeof current !== "object") {
      return undefined;
    }
    current = current[segment];
  }
  return current;
}

function setPathValue(model: Record<string, any>, path: string, value: any) {
  if (!model || !path) {
    return;
  }
  const segments = splitPath(path);
  if (segments.length === 0) {
    return;
  }
  let current: any = model;
  for (let i = 0; i < segments.length - 1; i += 1) {
    const segment = segments[i];
    if (!current[segment] || typeof current[segment] !== "object") {
      current[segment] = {};
    }
    current = current[segment];
  }
  current[segments[segments.length - 1]] = value;
}

function flexAlignment(value: any, fallback: string): string {
  const alignment = typeof value === "string" ? value : fallback;
  switch (alignment) {
    case "start":
      return "align-items:flex-start;";
    case "center":
      return "align-items:center;";
    case "end":
      return "align-items:flex-end;";
    case "stretch":
      return "align-items:stretch;";
    default:
      return `align-items:${alignment};`;
  }
}

function flexDistribution(value: any, fallback: string): string {
  const distribution = typeof value === "string" ? value : fallback;
  switch (distribution) {
    case "start":
      return "justify-content:flex-start;";
    case "center":
      return "justify-content:center;";
    case "end":
      return "justify-content:flex-end;";
    case "spaceBetween":
      return "justify-content:space-between;";
    case "spaceAround":
      return "justify-content:space-around;";
    case "spaceEvenly":
      return "justify-content:space-evenly;";
    default:
      return `justify-content:${distribution};`;
  }
}

function imageClass(usage: string): string {
  switch (usage) {
    case "icon":
      return "a2ui-image-icon";
    case "avatar":
      return "a2ui-image-avatar";
    case "smallFeature":
      return "a2ui-image-small";
    case "mediumFeature":
      return "a2ui-image-medium";
    case "largeFeature":
      return "a2ui-image-large";
    case "header":
      return "a2ui-image-header";
    default:
      return "a2ui-image-default";
  }
}

function objectFitClass(fit: string): string {
  switch (fit) {
    case "contain":
      return "a2ui-object-contain";
    case "fill":
      return "a2ui-object-fill";
    case "none":
      return "a2ui-object-none";
    case "scale-down":
      return "a2ui-object-scale";
    default:
      return "a2ui-object-cover";
  }
}

function renderFallback(message: string): string {
  return `<div class="a2ui-fallback">${escapeHtml(message)}</div>`;
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function escapeAttribute(value: string): string {
  return escapeHtml(value).replace(/`/g, "&#96;");
}

function isPlainObject(value: unknown): value is Record<string, any> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

const defaultStyles = `
:root { color-scheme: light; }
* { box-sizing: border-box; }
body { margin: 0; font-family: "Inter", "Segoe UI", system-ui, -apple-system, sans-serif; background: #f8fafc; color: #0f172a; }
.a2ui-root { padding: 16px; display: flex; flex-direction: column; gap: 16px; }
.a2ui-surface { background: #ffffff; border: 1px solid #e2e8f0; border-radius: 16px; padding: 16px; box-shadow: 0 1px 2px rgba(15, 23, 42, 0.05); }
.a2ui-surface-label { margin-bottom: 8px; font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.08em; color: #64748b; }
.a2ui-column, .a2ui-row, .a2ui-list { display: flex; gap: 12px; }
.a2ui-row { flex-wrap: wrap; }
.a2ui-list { flex-direction: column; }
.a2ui-card { border: 1px solid #e2e8f0; border-radius: 12px; padding: 12px; background: #f8fafc; }
.a2ui-text { margin: 0; }
.a2ui-text-h1 { font-size: 24px; font-weight: 600; }
.a2ui-text-h2 { font-size: 20px; font-weight: 600; }
.a2ui-text-h3 { font-size: 18px; font-weight: 600; }
.a2ui-text-h4 { font-size: 16px; font-weight: 600; }
.a2ui-text-h5 { font-size: 14px; font-weight: 600; }
.a2ui-text-body { font-size: 14px; }
.a2ui-text-caption { font-size: 12px; color: #64748b; }
.a2ui-image { max-width: 100%; border-radius: 12px; }
.a2ui-image-icon { height: 32px; width: 32px; }
.a2ui-image-avatar { height: 48px; width: 48px; border-radius: 999px; }
.a2ui-image-small { height: 140px; width: 100%; }
.a2ui-image-medium { height: 180px; width: 100%; }
.a2ui-image-large { height: 220px; width: 100%; }
.a2ui-image-header { height: 260px; width: 100%; }
.a2ui-image-default { width: 100%; }
.a2ui-object-cover { object-fit: cover; }
.a2ui-object-contain { object-fit: contain; }
.a2ui-object-fill { object-fit: fill; }
.a2ui-object-none { object-fit: none; }
.a2ui-object-scale { object-fit: scale-down; }
.a2ui-video { width: 100%; border-radius: 12px; border: 1px solid #e2e8f0; }
.a2ui-audio { display: flex; flex-direction: column; gap: 8px; }
.a2ui-audio-label { font-size: 13px; font-weight: 600; }
.a2ui-audio-player { width: 100%; }
.a2ui-button-row { display: flex; align-items: center; gap: 8px; }
.a2ui-button { border: 1px solid #e2e8f0; border-radius: 10px; background: #f1f5f9; padding: 8px 14px; font-size: 13px; font-weight: 600; color: #0f172a; }
.a2ui-button-primary { background: #0f172a; border-color: #0f172a; color: #f8fafc; }
.a2ui-badge { display: inline-flex; align-items: center; gap: 4px; border: 1px solid #e2e8f0; border-radius: 999px; padding: 2px 8px; font-size: 11px; font-weight: 600; color: #334155; background: #f8fafc; }
.a2ui-checkbox, .a2ui-option { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.a2ui-field { display: flex; flex-direction: column; gap: 6px; }
.a2ui-field-label { font-size: 13px; font-weight: 600; }
.a2ui-input { width: 100%; border: 1px solid #e2e8f0; border-radius: 10px; padding: 8px 10px; font-size: 13px; background: #ffffff; }
.a2ui-textarea { width: 100%; min-height: 100px; border: 1px solid #e2e8f0; border-radius: 10px; padding: 8px 10px; font-size: 13px; }
.a2ui-slider { display: flex; flex-direction: column; gap: 6px; }
.a2ui-slider-value { font-size: 12px; color: #64748b; }
.a2ui-tabs { display: flex; flex-direction: column; gap: 12px; }
.a2ui-tab { border: 1px solid #e2e8f0; border-radius: 12px; padding: 12px; background: #f8fafc; }
.a2ui-tab-title { font-size: 13px; font-weight: 600; margin-bottom: 8px; }
.a2ui-modal { border: 1px dashed #cbd5f5; border-radius: 12px; padding: 12px; background: #f8fafc; display: flex; flex-direction: column; gap: 8px; }
.a2ui-modal-entry { font-size: 13px; font-weight: 600; }
.a2ui-modal-content { border-top: 1px solid #e2e8f0; padding-top: 8px; }
.a2ui-divider { border: none; border-top: 1px solid #e2e8f0; margin: 8px 0; }
.a2ui-divider-vertical { width: 1px; height: 40px; background: #e2e8f0; }
.a2ui-fallback { border: 1px dashed #cbd5f5; border-radius: 10px; padding: 8px; font-size: 12px; color: #64748b; background: #f1f5f9; }
.a2ui-weighted { min-width: 0; }
`;
