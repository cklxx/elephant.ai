"use client";

import { useMemo } from "react";

import {
  Dialog,
  DialogContent,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import { A2UIMessage } from "@/lib/a2ui";

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

type RenderContext = {
  surface: A2UISurface;
  dataModel: Record<string, any>;
  basePath: string;
};

type A2UIState = {
  surfaces: Record<string, A2UISurface>;
  surfaceOrder: string[];
};

const DEFAULT_SURFACE_ID = "default";

const ALIGNMENT_CLASS: Record<string, string> = {
  start: "items-start",
  center: "items-center",
  end: "items-end",
  stretch: "items-stretch",
};

const DISTRIBUTION_CLASS: Record<string, string> = {
  start: "justify-start",
  center: "justify-center",
  end: "justify-end",
  spaceBetween: "justify-between",
  spaceAround: "justify-around",
  spaceEvenly: "justify-evenly",
};

const TEXT_HINT_CLASS: Record<string, string> = {
  h1: "text-2xl font-semibold",
  h2: "text-xl font-semibold",
  h3: "text-lg font-semibold",
  h4: "text-base font-semibold",
  h5: "text-sm font-semibold",
  caption: "text-xs text-muted-foreground",
  body: "text-sm text-foreground",
};

const IMAGE_HINT_CLASS: Record<string, string> = {
  icon: "h-8 w-8",
  avatar: "h-12 w-12 rounded-full",
  smallFeature: "h-28 w-full rounded-lg",
  mediumFeature: "h-40 w-full rounded-xl",
  largeFeature: "h-56 w-full rounded-2xl",
  header: "h-64 w-full rounded-2xl",
};

const OBJECT_FIT_CLASS: Record<string, string> = {
  cover: "object-cover",
  contain: "object-contain",
  fill: "object-fill",
  none: "object-none",
  "scale-down": "object-scale-down",
};

export function A2UIRenderer({
  messages,
  className,
}: {
  messages: A2UIMessage[];
  className?: string;
}) {
  const state = useMemo(() => buildState(messages), [messages]);
  if (state.surfaceOrder.length === 0) {
    return (
      <div className={cn("text-sm text-muted-foreground", className)}>
        No A2UI surfaces to render.
      </div>
    );
  }

  const showSurfaceLabel = state.surfaceOrder.length > 1;

  return (
    <div className={cn("space-y-4", className)}>
      {state.surfaceOrder.map((surfaceId) => {
        const surface = state.surfaces[surfaceId];
        if (!surface) {
          return null;
        }
        const rootId = surface.rootId;
        const content = rootId
          ? renderComponentById(surface, rootId, {
              surface,
              dataModel: surface.dataModel,
              basePath: "/",
            })
          : renderFallback("Surface missing root component.");

        return (
          <div
            key={`a2ui-surface-${surfaceId}`}
            className="rounded-xl border border-border/50 bg-background/70 p-4"
          >
            {showSurfaceLabel && (
              <div className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {surfaceId}
              </div>
            )}
            {content}
          </div>
        );
      })}
    </div>
  );
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

function applyDataModelUpdate(surface: A2UISurface, update: NonNullable<A2UIMessage["dataModelUpdate"]>) {
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

function renderComponentById(
  surface: A2UISurface,
  componentId: string,
  context: RenderContext,
  key?: string,
) {
  const component = surface.components[componentId];
  if (!component) {
    return renderFallback(`Missing component: ${componentId}`, key);
  }
  return renderComponent(surface, component, context, key);
}

function renderComponent(
  surface: A2UISurface,
  component: A2UIComponent,
  context: RenderContext,
  key?: string,
): JSX.Element {
  const props = component.props || {};
  switch (component.type) {
    case "Column": {
      const children = renderChildren(surface, props.children, context, component.id);
      const alignment = ALIGNMENT_CLASS[props.alignment] || "items-stretch";
      const distribution = DISTRIBUTION_CLASS[props.distribution] || "justify-start";
      return (
        <div
          key={key ?? component.id}
          className={cn("flex flex-col gap-3", alignment, distribution)}
        >
          {children}
        </div>
      );
    }
    case "Row": {
      const children = renderChildren(surface, props.children, context, component.id);
      const alignment = ALIGNMENT_CLASS[props.alignment] || "items-start";
      const distribution = DISTRIBUTION_CLASS[props.distribution] || "justify-start";
      return (
        <div
          key={key ?? component.id}
          className={cn("flex flex-row flex-wrap gap-3", alignment, distribution)}
        >
          {children}
        </div>
      );
    }
    case "List": {
      const children = renderChildren(surface, props.children, context, component.id);
      const direction = props.direction === "horizontal" ? "flex-row" : "flex-col";
      const alignment = ALIGNMENT_CLASS[props.alignment] || "items-stretch";
      return (
        <div
          key={key ?? component.id}
          className={cn("flex gap-3", direction, alignment)}
        >
          {children}
        </div>
      );
    }
    case "Card": {
      const childId = typeof props.child === "string" ? props.child : "";
      return (
        <div
          key={key ?? component.id}
          className="rounded-xl border border-border/60 bg-card p-4"
        >
          {childId
            ? renderComponentById(surface, childId, context, `${component.id}-child`)
            : renderFallback("Card missing child.")}
        </div>
      );
    }
    case "Text": {
      const value = resolveBoundValue(props.text, context);
      const usageHint = typeof props.usageHint === "string" ? props.usageHint : "body";
      const textClass = TEXT_HINT_CLASS[usageHint] || TEXT_HINT_CLASS.body;
      const Tag: any = usageHint.startsWith("h") ? usageHint : "p";
      return (
        <Tag key={key ?? component.id} className={textClass}>
          {value ?? ""}
        </Tag>
      );
    }
    case "Image": {
      const url = resolveBoundValue(props.url, context);
      if (!url) {
        return renderFallback("Image missing url.", key);
      }
      const usageHint = typeof props.usageHint === "string" ? props.usageHint : "";
      const hintClass = IMAGE_HINT_CLASS[usageHint] || "w-full rounded-lg";
      const fitClass = OBJECT_FIT_CLASS[props.fit] || "object-cover";
      return (
        <img
          key={key ?? component.id}
          src={String(url)}
          alt=""
          className={cn("max-w-full", hintClass, fitClass)}
        />
      );
    }
    case "Video": {
      const url = resolveBoundValue(props.url, context);
      if (!url) {
        return renderFallback("Video missing url.", key);
      }
      return (
        <video
          key={key ?? component.id}
          src={String(url)}
          controls
          className="w-full rounded-lg border border-border/60"
        />
      );
    }
    case "AudioPlayer": {
      const url = resolveBoundValue(props.url, context);
      const description = resolveBoundValue(props.description, context);
      return (
        <div key={key ?? component.id} className="space-y-2">
          {description && (
            <div className="text-sm font-medium text-foreground">{description}</div>
          )}
          {url ? (
            <audio src={String(url)} controls className="w-full" />
          ) : (
            renderFallback("Audio missing url.")
          )}
        </div>
      );
    }
    case "Button": {
      const childId = typeof props.child === "string" ? props.child : "";
      const actionName = props.action?.name;
      const isPrimary = Boolean(props.primary);
      const variant = isPrimary ? "default" : "secondary";
      return (
        <div key={key ?? component.id} className="flex items-center gap-2">
          <Button variant={variant} disabled={!childId}>
            {childId
              ? renderComponentById(surface, childId, context, `${component.id}-label`)
              : "Button"}
          </Button>
          {actionName && <Badge variant="outline">{actionName}</Badge>}
        </div>
      );
    }
    case "CheckBox": {
      const label = resolveBoundValue(props.label, context);
      const value = resolveBoundValue(props.value, context);
      return (
        <label
          key={key ?? component.id}
          className="flex items-center gap-2 text-sm text-foreground"
        >
          <input type="checkbox" checked={Boolean(value)} readOnly />
          <span>{label ?? ""}</span>
        </label>
      );
    }
    case "TextField": {
      const label = resolveBoundValue(props.label, context);
      const value = resolveBoundValue(props.text, context);
      const fieldType = typeof props.textFieldType === "string" ? props.textFieldType : "shortText";
      const inputType = fieldType === "obscured" ? "password" : fieldType === "number" ? "number" : fieldType === "date" ? "date" : "text";
      return (
        <div key={key ?? component.id} className="space-y-2">
          {label && <label className="text-sm font-medium text-foreground">{label}</label>}
          {fieldType === "longText" ? (
            <Textarea value={value ?? ""} readOnly />
          ) : (
            <Input type={inputType} value={value ?? ""} readOnly />
          )}
        </div>
      );
    }
    case "DateTimeInput": {
      const value = resolveBoundValue(props.value, context);
      const enableDate = props.enableDate !== false;
      const enableTime = props.enableTime !== false;
      const inputType = enableDate && enableTime ? "datetime-local" : enableTime ? "time" : "date";
      return (
        <Input
          key={key ?? component.id}
          type={inputType}
          value={value ?? ""}
          readOnly
        />
      );
    }
    case "Slider": {
      const value = resolveBoundValue(props.value, context);
      return (
        <div key={key ?? component.id} className="space-y-2">
          <input
            type="range"
            min={props.minValue ?? 0}
            max={props.maxValue ?? 100}
            value={typeof value === "number" ? value : 0}
            readOnly
            className="w-full"
          />
          <div className="text-xs text-muted-foreground">
            Value: {value ?? 0}
          </div>
        </div>
      );
    }
    case "MultipleChoice": {
      const selections = resolveBoundValue(props.selections, context);
      const selectionList = Array.isArray(selections) ? selections : selections ? [selections] : [];
      const options = Array.isArray(props.options) ? props.options : [];
      const maxAllowed = typeof props.maxAllowedSelections === "number" ? props.maxAllowedSelections : undefined;
      const inputType = maxAllowed === 1 ? "radio" : "checkbox";
      return (
        <div key={key ?? component.id} className="space-y-2">
          {options.map((option: any) => {
            const label = resolveBoundValue(option.label, context);
            const optionValue = option.value;
            const checked = selectionList.includes(optionValue);
            return (
              <label
                key={`${component.id}-${optionValue}`}
                className="flex items-center gap-2 text-sm text-foreground"
              >
                <input type={inputType} checked={checked} readOnly />
                <span>{label ?? optionValue}</span>
              </label>
            );
          })}
        </div>
      );
    }
    case "Tabs": {
      const tabItems = Array.isArray(props.tabItems) ? props.tabItems : [];
      if (tabItems.length === 0) {
        return renderFallback("Tabs missing items.", key);
      }
      const defaultValue = `tab-${component.id}-0`;
      return (
        <Tabs key={key ?? component.id} defaultValue={defaultValue}>
          <TabsList>
            {tabItems.map((item: any, idx: number) => {
              const title = resolveBoundValue(item.title, context) ?? `Tab ${idx + 1}`;
              return (
                <TabsTrigger key={`tab-trigger-${idx}`} value={`tab-${component.id}-${idx}`}>
                  {title}
                </TabsTrigger>
              );
            })}
          </TabsList>
          {tabItems.map((item: any, idx: number) => {
            const childId = typeof item.child === "string" ? item.child : "";
            return (
              <TabsContent key={`tab-content-${idx}`} value={`tab-${component.id}-${idx}`}>
                {childId
                  ? renderComponentById(surface, childId, context, `${component.id}-tab-${idx}`)
                  : renderFallback("Tab missing child.")}
              </TabsContent>
            );
          })}
        </Tabs>
      );
    }
    case "Modal": {
      const entryId = typeof props.entryPointChild === "string" ? props.entryPointChild : "";
      const contentId = typeof props.contentChild === "string" ? props.contentChild : "";
      if (!entryId || !contentId) {
        return renderFallback("Modal missing entryPointChild or contentChild.", key);
      }
      return (
        <Dialog key={key ?? component.id}>
          <DialogTrigger asChild>
            <span className="inline-flex">
              {renderComponentById(surface, entryId, context, `${component.id}-entry`)}
            </span>
          </DialogTrigger>
          <DialogContent>
            {renderComponentById(surface, contentId, context, `${component.id}-content`)}
          </DialogContent>
        </Dialog>
      );
    }
    case "Divider": {
      const axis = props.axis === "vertical" ? "vertical" : "horizontal";
      return axis === "vertical" ? (
        <div key={key ?? component.id} className="h-10 w-px bg-border" />
      ) : (
        <hr key={key ?? component.id} className="my-2 border-border" />
      );
    }
    case "Icon": {
      const name = resolveBoundValue(props.name, context);
      return (
        <Badge key={key ?? component.id} variant="outline">
          {name ?? "icon"}
        </Badge>
      );
    }
    default:
      return renderFallback(`Unsupported component: ${component.type}`, key);
  }
}

function renderChildren(
  surface: A2UISurface,
  childrenDef: any,
  context: RenderContext,
  parentKey: string,
) {
  if (!childrenDef) {
    return null;
  }

  if (Array.isArray(childrenDef)) {
    return childrenDef.map((childId, idx) =>
      renderWeightedChild(surface, childId, context, `${parentKey}-${idx}`),
    );
  }

  if (childrenDef.explicitList && Array.isArray(childrenDef.explicitList)) {
    return childrenDef.explicitList.map((childId: string, idx: number) =>
      renderWeightedChild(surface, childId, context, `${parentKey}-${idx}`),
    );
  }

  if (childrenDef.template && childrenDef.template.componentId) {
    const bindingPath = resolveBindingPath(childrenDef.template.dataBinding, context.basePath);
    const data = getPathValue(context.dataModel, bindingPath);
    if (data && typeof data === "object") {
      const entries = Array.isArray(data) ? data.map((_, idx) => String(idx)) : Object.keys(data);
      return entries.map((key) => {
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
      });
    }
    return renderFallback("Template data binding empty.");
  }

  return null;
}

function renderWeightedChild(
  surface: A2UISurface,
  childId: string,
  context: RenderContext,
  key: string,
) {
  if (typeof childId !== "string") {
    return renderFallback("Invalid child reference.", key);
  }
  const component = surface.components[childId];
  const weight = component?.weight;
  const content = renderComponentById(surface, childId, context, key);
  const style = weight ? { flexGrow: weight } : undefined;
  return (
    <div key={key} className="min-w-0" style={style}>
      {content}
    </div>
  );
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

function renderFallback(message: string, key?: string) {
  return (
    <div
      key={key ?? message}
      className="rounded-lg border border-dashed border-border/60 bg-muted/20 p-3 text-xs text-muted-foreground"
    >
      {message}
    </div>
  );
}

function isPlainObject(value: unknown): value is Record<string, any> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}
