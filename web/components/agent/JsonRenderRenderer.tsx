import Image from "next/image";
import type { CSSProperties, JSX } from "react";

import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { JsonRenderElement, JsonRenderTree } from "@/lib/json-render-model";

export function JsonRenderRenderer({ tree }: { tree: JsonRenderTree }) {
  if (!tree.root) {
    return (
      <div className="text-sm text-muted-foreground">
        No json-render UI content.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {renderElement(tree.root, tree, tree.root.key ?? "root")}
    </div>
  );
}

function renderElement(
  element: JsonRenderElement,
  tree: JsonRenderTree,
  key: string,
): JSX.Element {
  const type = element.type.toLowerCase();
  const props = element.props ?? {};

  switch (type) {
    case "column": {
      const alignment = alignClass(props.align);
      const distribution = justifyClass(props.justify);
      return (
        <div key={key} className={cn("flex flex-col gap-3", alignment, distribution)}>
          {renderChildren(element, tree, key)}
        </div>
      );
    }
    case "row": {
      const alignment = alignClass(props.align);
      const distribution = justifyClass(props.justify);
      return (
        <div key={key} className={cn("flex flex-row flex-wrap gap-3", alignment, distribution)}>
          {renderChildren(element, tree, key)}
        </div>
      );
    }
    case "list": {
      return (
        <div key={key} className="flex flex-col gap-3">
          {renderChildren(element, tree, key)}
        </div>
      );
    }
    case "card": {
      const padding = toCssSize(props.padding);
      const radius = toCssSize(props.radius);
      const style: CSSProperties = {};
      if (padding) {
        style.padding = padding;
      }
      if (radius) {
        style.borderRadius = radius;
      }
      const borderClass =
        props.border === false ? "border-0" : "border border-border/60";
      return (
        <div
          key={key}
          className={cn("rounded-xl bg-card", borderClass)}
          style={style}
        >
          {renderChildren(element, tree, key)}
        </div>
      );
    }
    case "heading": {
      const text = String(props.text ?? props.title ?? "");
      const level = clampHeadingLevel(props.level);
      const Tag: any = `h${level}`;
      const className = headingClass(level);
      return (
        <Tag key={key} className={className}>
          {text}
        </Tag>
      );
    }
    case "text":
    case "paragraph": {
      const text = String(props.text ?? props.value ?? "");
      const style: CSSProperties = {};
      const color = toCssColor(props.color);
      if (color) {
        style.color = color;
      }
      const marginTop = toCssSize(props.marginTop);
      if (marginTop) {
        style.marginTop = marginTop;
      }
      const align = textAlignValue(props.align);
      if (align) {
        style.textAlign = align;
      }
      return (
        <p
          key={key}
          className={cn(
            "text-sm text-foreground",
            textSizeClass(props.size),
            textWeightClass(props.weight),
          )}
          style={style}
        >
          {text}
        </p>
      );
    }
    case "badge": {
      const text = String(props.text ?? props.value ?? "");
      const style = badgeStyleFromProps(props);
      return (
        <Badge
          key={key}
          variant="outline"
          style={style}
          className={textSizeClass(props.size)}
        >
          {text}
        </Badge>
      );
    }
    case "tag": {
      const text = String(props.value ?? props.text ?? "");
      const style = badgeStyleFromProps(props);
      return (
        <Badge
          key={key}
          variant="outline"
          style={style}
          className={textSizeClass(props.size)}
        >
          {text}
        </Badge>
      );
    }
    case "divider": {
      return <hr key={key} className="my-2 border-border" />;
    }
    case "image": {
      const url = String(props.url ?? props.src ?? "");
      return url ? (
        <Image
          key={key}
          src={url}
          alt=""
          width={resolveImageSize(props.ratio).width}
          height={resolveImageSize(props.ratio).height}
          unoptimized
          className={cn("h-auto w-full object-cover", imageBorderClass(props))}
          style={imageStyleFromProps(props)}
        />
      ) : (
        <Fallback key={key} message="Image missing url." />
      );
    }
    case "container": {
      const style = layoutStyleFromProps(props);
      return (
        <div
          key={key}
          className={cn("flex flex-col", alignClass(props.align), justifyClass(props.justify))}
          style={style}
        >
          {renderChildren(element, tree, key)}
        </div>
      );
    }
    case "grid": {
      const style = gridStyleFromProps(props);
      return (
        <div key={key} className="grid" style={style}>
          {renderChildren(element, tree, key)}
        </div>
      );
    }
    case "flow": {
      const nodes = Array.isArray(props.nodes) ? props.nodes : [];
      const edges = Array.isArray(props.edges) ? props.edges : [];
      const direction = props.direction === "vertical" ? "vertical" : "horizontal";
      return (
        <div
          key={key}
          className={cn(
            "flex flex-wrap items-center gap-2",
            direction === "vertical" && "flex-col items-start",
          )}
        >
          {renderFlow(nodes, edges, direction)}
        </div>
      );
    }
    case "form": {
      const title = props.title ? String(props.title) : "";
      const description = props.description ? String(props.description) : "";
      const fields = Array.isArray(props.fields) ? props.fields : [];
      return (
        <div key={key} className="space-y-3">
          {title ? <h3 className="text-base font-semibold">{title}</h3> : null}
          {description ? (
            <p className="text-xs text-muted-foreground">{description}</p>
          ) : null}
          {fields.map((field: any, index: number) => (
            <FieldRow key={`${key}-field-${index}`} field={field} />
          ))}
        </div>
      );
    }
    case "dashboard": {
      const title = props.title ? String(props.title) : "";
      const metrics = Array.isArray(props.metrics) ? props.metrics : [];
      const items = Array.isArray(props.items) ? props.items : [];
      return (
        <div key={key} className="space-y-3">
          {title ? <h3 className="text-base font-semibold">{title}</h3> : null}
          <div className="flex flex-wrap gap-3">
            {metrics.map((metric: any, index: number) => (
              <MetricCard key={`${key}-metric-${index}`} metric={metric} />
            ))}
          </div>
          {items.length > 0 ? (
            <div className="space-y-2">
              {items.map((item: any, index: number) => (
                <div
                  key={`${key}-item-${index}`}
                  className="rounded-lg border border-border/60 bg-muted/20 px-3 py-2 text-sm"
                >
                  <div className="font-medium text-foreground">
                    {String(item.title ?? item.label ?? "")}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    {String(item.meta ?? item.caption ?? "")}
                  </div>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      );
    }
    case "info_cards":
    case "cards": {
      const items = Array.isArray(props.items) ? props.items : [];
      return (
        <div key={key} className="grid gap-3 sm:grid-cols-2">
          {items.map((item: any, index: number) => (
            <div
              key={`${key}-card-${index}`}
              className="rounded-xl border border-border/60 bg-card p-4"
            >
              <div className="text-sm font-semibold text-foreground">
                {String(item.title ?? "")}
              </div>
              {item.subtitle ? (
                <div className="text-xs text-muted-foreground">
                  {String(item.subtitle)}
                </div>
              ) : null}
              {item.body ? (
                <div className="mt-2 text-sm text-foreground">
                  {String(item.body)}
                </div>
              ) : null}
            </div>
          ))}
        </div>
      );
    }
    case "gallery": {
      const items = Array.isArray(props.items) ? props.items : [];
      return (
        <div key={key} className="flex flex-wrap gap-3">
          {items.map((item: any, index: number) => (
            <div
              key={`${key}-gallery-${index}`}
              className="w-full max-w-[220px] space-y-2"
            >
              <div className="overflow-hidden rounded-lg border border-border/60">
                <Image
                  src={String(item.url ?? "")}
                  alt=""
                  width={220}
                  height={140}
                  unoptimized
                  className="h-[140px] w-full object-cover"
                />
              </div>
              <div className="text-xs text-muted-foreground">
                {String(item.caption ?? "")}
              </div>
            </div>
          ))}
        </div>
      );
    }
    case "table": {
      const rows = Array.isArray(props.rows) ? props.rows : [];
      const headers = normalizeTableHeaders(props.headers, rows);
      const tableRows = normalizeTableRows(rows, headers);
      if (tableRows.length === 0) {
        return <Fallback key={key} message="Table is empty." />;
      }
      return (
        <div key={key} className="overflow-auto rounded-xl border border-border/60">
          <table className="w-full text-sm">
            <thead className="bg-muted/30 text-xs uppercase text-muted-foreground">
              <tr>
                {headers.map((header) => (
                  <th key={`${key}-th-${header}`} className="px-3 py-2 text-left">
                    {header}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {tableRows.map((row, rowIndex) => (
                <tr key={`${key}-row-${rowIndex}`} className="border-t border-border/40">
                  {row.map((cell, cellIndex) => (
                    <td key={`${key}-cell-${rowIndex}-${cellIndex}`} className="px-3 py-2">
                      {cell}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );
    }
    case "kanban": {
      const columns = Array.isArray(props.columns) ? props.columns : [];
      if (columns.length === 0) {
        return <Fallback key={key} message="Kanban has no columns." />;
      }
      return (
        <div key={key} className="flex gap-4 overflow-auto pb-2">
          {columns.map((column: any, index: number) => (
            <div
              key={`${key}-col-${index}`}
              className="min-w-[200px] rounded-xl border border-border/60 bg-muted/20 p-3"
            >
              <div className="text-sm font-semibold text-foreground">
                {String(column?.title ?? `Column ${index + 1}`)}
              </div>
              <div className="mt-2 space-y-2">
                {normalizeKanbanItems(column?.items).map((item, itemIndex) => (
                  <div
                    key={`${key}-item-${index}-${itemIndex}`}
                    className="rounded-lg border border-border/60 bg-card px-3 py-2 text-sm"
                  >
                    <div className="font-medium text-foreground">{item.title}</div>
                    {item.subtitle ? (
                      <div className="text-xs text-muted-foreground">{item.subtitle}</div>
                    ) : null}
                    {item.meta ? (
                      <div className="text-xs text-muted-foreground">{item.meta}</div>
                    ) : null}
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      );
    }
    case "diagram": {
      const nodes = Array.isArray(props.nodes) ? props.nodes : [];
      const edges = Array.isArray(props.edges) ? props.edges : [];
      if (nodes.length === 0 && edges.length === 0) {
        return <Fallback key={key} message="Diagram has no nodes or edges." />;
      }
      return (
        <div key={key} className="space-y-2">
          {nodes.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {nodes.map((node: any, index: number) => (
                <div
                  key={`${key}-node-${node?.id ?? index}`}
                  className="rounded-lg border border-border/60 bg-muted/20 px-3 py-1 text-sm font-medium"
                >
                  {String(node?.label ?? node?.id ?? `Node ${index + 1}`)}
                </div>
              ))}
            </div>
          ) : null}
          {edges.length > 0 ? (
            <div className="space-y-1 text-xs text-muted-foreground">
              {edges.map((edge: any, index: number) => (
                <div key={`${key}-edge-${index}`}>
                  {String(edge?.from ?? "?")} -&gt; {String(edge?.to ?? "?")}
                  {edge?.label ? ` (${edge.label})` : ""}
                </div>
              ))}
            </div>
          ) : null}
        </div>
      );
    }
    default: {
      return <Fallback key={key} message={`Unsupported element: ${element.type}`} />;
    }
  }
}

function renderChildren(element: JsonRenderElement, tree: JsonRenderTree, key: string) {
  const children = Array.isArray(element.children) ? element.children : [];
  return children
    .map((child, idx) => {
      if (typeof child === "string") {
        const resolved = tree.elements[child];
        if (!resolved) {
          return (
            <Fallback
              key={`${key}-missing-${idx}`}
              message={`Missing element: ${child}`}
            />
          );
        }
        return renderElement(resolved, tree, `${key}-${child}-${idx}`);
      }
      return renderElement(child, tree, child.key ?? `${key}-${idx}`);
    })
    .filter(Boolean);
}

function renderFlow(nodes: any[], edges: any[], direction: string) {
  const order = nodes.length > 0 ? nodes : deriveNodesFromEdges(edges);
  const arrow = direction === "vertical" ? "v" : "->";

  return order.flatMap((node: any, index: number) => {
    const parts: JSX.Element[] = [
      <div
        key={`node-${node.id ?? index}`}
        className="rounded-lg border border-border/60 bg-muted/20 px-3 py-2 text-sm font-medium"
      >
        {String(node.label ?? node.id ?? `Node ${index + 1}`)}
      </div>,
    ];
    if (index < order.length - 1) {
      const edge = findEdge(edges, node.id, order[index + 1]?.id);
      parts.push(
        <span key={`arrow-${index}`} className="text-xs text-muted-foreground">
          {edge?.label ? `${arrow} ${edge.label}` : arrow}
        </span>,
      );
    }
    return parts;
  });
}

function deriveNodesFromEdges(edges: any[]) {
  const ids: string[] = [];
  edges.forEach((edge) => {
    if (edge?.from && !ids.includes(edge.from)) {
      ids.push(edge.from);
    }
    if (edge?.to && !ids.includes(edge.to)) {
      ids.push(edge.to);
    }
  });
  return ids.map((id) => ({ id, label: id }));
}

function findEdge(edges: any[], from: any, to: any) {
  return edges.find((edge) => edge?.from === from && edge?.to === to);
}

function toCssSize(value: any): string | undefined {
  if (typeof value === "number" && Number.isFinite(value)) {
    return `${value}px`;
  }
  if (typeof value === "string" && value.trim() !== "") {
    return value;
  }
  return undefined;
}

function toCssColor(value: any): string | undefined {
  if (typeof value === "string" && value.trim() !== "") {
    return value;
  }
  return undefined;
}

function textSizeClass(value: any): string {
  if (typeof value !== "string") {
    return "";
  }
  switch (value.toLowerCase()) {
    case "xs":
      return "text-xs";
    case "sm":
      return "text-sm";
    case "md":
      return "text-base";
    case "lg":
      return "text-lg";
    case "xl":
      return "text-xl";
    case "2xl":
      return "text-2xl";
    default:
      return "";
  }
}

function textWeightClass(value: any): string {
  if (typeof value !== "string") {
    return "";
  }
  switch (value.toLowerCase()) {
    case "bold":
    case "semibold":
      return "font-semibold";
    case "medium":
      return "font-medium";
    case "normal":
      return "font-normal";
    default:
      return "";
  }
}

function textAlignValue(value: any): CSSProperties["textAlign"] | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  switch (value.toLowerCase()) {
    case "center":
      return "center";
    case "right":
      return "right";
    case "left":
      return "left";
    default:
      return undefined;
  }
}

function badgeStyleFromProps(props: Record<string, any>): CSSProperties {
  const style: CSSProperties = {};
  const color = toCssColor(props.color);
  if (color) {
    style.color = color;
    style.borderColor = color;
  }
  const marginTop = toCssSize(props.marginTop);
  if (marginTop) {
    style.marginTop = marginTop;
  }
  return style;
}

function layoutStyleFromProps(props: Record<string, any>): CSSProperties {
  const style: CSSProperties = {};
  const gap = toCssSize(props.gap);
  if (gap) {
    style.gap = gap;
  }
  const padding = toCssSize(props.padding);
  if (padding) {
    style.padding = padding;
  }
  return style;
}

function gridStyleFromProps(props: Record<string, any>): CSSProperties {
  const style: CSSProperties = layoutStyleFromProps(props);
  const columns =
    typeof props.columns === "number" && Number.isFinite(props.columns)
      ? props.columns
      : typeof props.columns === "string"
        ? parseInt(props.columns, 10)
        : 0;
  if (columns > 0) {
    style.gridTemplateColumns = `repeat(${columns}, minmax(0, 1fr))`;
  }
  return style;
}

function resolveImageSize(ratio: any): { width: number; height: number } {
  const fallback = { width: 800, height: 420 };
  if (typeof ratio !== "string") {
    return fallback;
  }
  const parts = ratio.split(":");
  if (parts.length !== 2) {
    return fallback;
  }
  const width = Number(parts[0]);
  const height = Number(parts[1]);
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
    return fallback;
  }
  const baseWidth = 800;
  return { width: baseWidth, height: Math.round(baseWidth * (height / width)) };
}

function imageStyleFromProps(props: Record<string, any>): CSSProperties {
  const style: CSSProperties = {};
  const radius = toCssSize(props.radius);
  if (radius) {
    style.borderRadius = radius;
  }
  return style;
}

function imageBorderClass(props: Record<string, any>): string {
  return props.border ? "border border-border/60" : "";
}

function FieldRow({ field }: { field: any }) {
  const label = field?.label ? String(field.label) : "";
  const value = field?.value ?? "";
  const placeholder = field?.placeholder ? String(field.placeholder) : "";
  const rawType = field?.type ? String(field.type) : "text";
  const type = rawType === "textarea" ? "textarea" : "input";
  const inputType = rawType === "textarea" ? "text" : rawType;
  const rows =
    typeof field?.rows === "number" && Number.isFinite(field.rows)
      ? field.rows
      : undefined;
  return (
    <div className="space-y-1">
      {label ? <div className="text-xs font-medium text-foreground">{label}</div> : null}
      {type === "textarea" ? (
        <Textarea value={value} placeholder={placeholder} rows={rows} readOnly />
      ) : (
        <Input
          type={inputType}
          value={value}
          placeholder={placeholder}
          readOnly
        />
      )}
    </div>
  );
}

function MetricCard({ metric }: { metric: any }) {
  const label = metric?.label ? String(metric.label) : "";
  const value = metric?.value ?? "";
  const trend = metric?.trend ? String(metric.trend) : "";
  return (
    <div className="min-w-[160px] rounded-xl border border-border/60 bg-card p-4">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="text-lg font-semibold text-foreground">{value}</div>
      {trend ? <div className="text-xs text-muted-foreground">{trend}</div> : null}
    </div>
  );
}

function normalizeTableHeaders(headers: any, rows: any[]): string[] {
  if (Array.isArray(headers) && headers.length > 0) {
    return headers.map((header) => String(header));
  }
  if (rows.length > 0 && isPlainObject(rows[0])) {
    return Object.keys(rows[0]);
  }
  return ["Value"];
}

function normalizeTableRows(rows: any[], headers: string[]): string[][] {
  return rows.map((row) => {
    if (Array.isArray(row)) {
      const normalized = row.map((cell) => String(cell ?? ""));
      return normalized.length > 0 ? normalized : [""];
    }
    if (isPlainObject(row)) {
      return headers.map((header) => String(row[header] ?? ""));
    }
    return [String(row ?? "")];
  });
}

function normalizeKanbanItems(items: any): Array<{ title: string; subtitle: string; meta: string }> {
  if (!Array.isArray(items)) {
    return [];
  }
  return items.map((item) => {
    if (isPlainObject(item)) {
      return {
        title: String(item.title ?? item.label ?? item.name ?? "Untitled"),
        subtitle: item.subtitle ? String(item.subtitle) : "",
        meta: item.meta ? String(item.meta) : "",
      };
    }
    return { title: String(item ?? ""), subtitle: "", meta: "" };
  });
}

function headingClass(level: number) {
  switch (level) {
    case 1:
      return "text-2xl font-semibold";
    case 2:
      return "text-xl font-semibold";
    case 3:
      return "text-lg font-semibold";
    default:
      return "text-base font-semibold";
  }
}

function clampHeadingLevel(value: any): number {
  const level = typeof value === "number" ? value : parseInt(String(value ?? ""), 10);
  if (Number.isNaN(level)) {
    return 2;
  }
  return Math.min(4, Math.max(1, level));
}

function isPlainObject(value: unknown): value is Record<string, any> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function alignClass(value: any) {
  switch (value) {
    case "center":
      return "items-center";
    case "end":
      return "items-end";
    case "stretch":
      return "items-stretch";
    default:
      return "items-start";
  }
}

function justifyClass(value: any) {
  switch (value) {
    case "center":
      return "justify-center";
    case "end":
      return "justify-end";
    case "between":
      return "justify-between";
    default:
      return "justify-start";
  }
}

function Fallback({ message }: { message: string }) {
  return (
    <div className="rounded-lg border border-dashed border-border/60 bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
      {message}
    </div>
  );
}
