import { JsonRenderElement, JsonRenderTree } from "@/lib/json-render-model";

export function renderJsonRenderHtml(tree: JsonRenderTree): string {
  const root = tree.root;
  const parts: string[] = [];
  parts.push("<!doctype html><html><head>");
  parts.push('<meta charset="utf-8" />');
  parts.push('<meta name="viewport" content="width=device-width, initial-scale=1" />');
  parts.push("<style>");
  parts.push(JR_STYLES);
  parts.push("</style></head><body>");
  parts.push('<div class="jr-root">');
  if (root) {
    parts.push(renderElement(root, tree));
  } else {
    parts.push(renderFallback("No json-render content."));
  }
  parts.push("</div></body></html>");
  return parts.join("");
}

function renderElement(element: JsonRenderElement, tree: JsonRenderTree): string {
  const type = element.type.toLowerCase();
  const props = element.props ?? {};

  switch (type) {
    case "column": {
      const alignment = flexAlign(props.align, "flex-start");
      const distribution = flexJustify(props.justify, "flex-start");
      return `<div class="jr-column" style="${alignment}${distribution}">${renderChildren(
        element,
        tree,
      )}</div>`;
    }
    case "row": {
      const alignment = flexAlign(props.align, "flex-start");
      const distribution = flexJustify(props.justify, "flex-start");
      return `<div class="jr-row" style="${alignment}${distribution}">${renderChildren(
        element,
        tree,
      )}</div>`;
    }
    case "list": {
      return `<div class="jr-list">${renderChildren(element, tree)}</div>`;
    }
    case "card": {
      return `<div class="jr-card">${renderChildren(element, tree)}</div>`;
    }
    case "heading": {
      const text = escapeHtml(String(props.text ?? props.title ?? ""));
      const level = clampHeadingLevel(props.level);
      const tag = `h${level}`;
      return `<${tag} class="jr-heading jr-heading-${level}">${text}</${tag}>`;
    }
    case "text":
    case "paragraph": {
      const text = escapeHtml(String(props.text ?? props.value ?? ""));
      return `<p class="jr-text">${text}</p>`;
    }
    case "badge": {
      const text = escapeHtml(String(props.text ?? props.value ?? ""));
      return `<span class="jr-badge">${text}</span>`;
    }
    case "divider": {
      return '<hr class="jr-divider" />';
    }
    case "image": {
      const url = escapeAttribute(String(props.url ?? ""));
      if (!url) {
        return renderFallback("Image missing url.");
      }
      return `<img class="jr-image" src="${url}" alt="" />`;
    }
    case "flow": {
      const nodes = Array.isArray(props.nodes) ? props.nodes : [];
      const edges = Array.isArray(props.edges) ? props.edges : [];
      const direction = props.direction === "vertical" ? "vertical" : "horizontal";
      return renderFlow(nodes, edges, direction);
    }
    case "form": {
      const title = props.title ? escapeHtml(String(props.title)) : "";
      const fields = Array.isArray(props.fields) ? props.fields : [];
      const output: string[] = [];
      output.push('<div class="jr-form">');
      if (title) {
        output.push(`<div class="jr-section-title">${title}</div>`);
      }
      fields.forEach((field) => {
        const label = field?.label ? escapeHtml(String(field.label)) : "";
        const value = escapeAttribute(String(field?.value ?? ""));
        if (label) {
          output.push(`<div class="jr-field-label">${label}</div>`);
        }
        if (field?.type === "textarea") {
          output.push(`<textarea class="jr-textarea" readonly>${escapeHtml(String(field?.value ?? ""))}</textarea>`);
        } else {
          output.push(`<input class="jr-input" type="text" value="${value}" readonly />`);
        }
      });
      output.push("</div>");
      return output.join("");
    }
    case "dashboard": {
      const title = props.title ? escapeHtml(String(props.title)) : "";
      const metrics = Array.isArray(props.metrics) ? props.metrics : [];
      const items = Array.isArray(props.items) ? props.items : [];
      const output: string[] = [];
      output.push('<div class="jr-dashboard">');
      if (title) {
        output.push(`<div class="jr-section-title">${title}</div>`);
      }
      output.push('<div class="jr-metrics">');
      metrics.forEach((metric) => {
        output.push('<div class="jr-metric">');
        output.push(`<div class="jr-metric-label">${escapeHtml(String(metric?.label ?? ""))}</div>`);
        output.push(`<div class="jr-metric-value">${escapeHtml(String(metric?.value ?? ""))}</div>`);
        if (metric?.trend) {
          output.push(`<div class="jr-metric-trend">${escapeHtml(String(metric.trend))}</div>`);
        }
        output.push("</div>");
      });
      output.push("</div>");
      if (items.length > 0) {
        output.push('<div class="jr-list">');
        items.forEach((item) => {
          output.push('<div class="jr-list-item">');
          output.push(`<div class="jr-list-title">${escapeHtml(String(item?.title ?? item?.label ?? ""))}</div>`);
          output.push(`<div class="jr-list-meta">${escapeHtml(String(item?.meta ?? item?.caption ?? ""))}</div>`);
          output.push("</div>");
        });
        output.push("</div>");
      }
      output.push("</div>");
      return output.join("");
    }
    case "info_cards":
    case "cards": {
      const items = Array.isArray(props.items) ? props.items : [];
      const output: string[] = [];
      output.push('<div class="jr-card-grid">');
      items.forEach((item) => {
        output.push('<div class="jr-card">');
        output.push(`<div class="jr-card-title">${escapeHtml(String(item?.title ?? ""))}</div>`);
        if (item?.subtitle) {
          output.push(`<div class="jr-card-subtitle">${escapeHtml(String(item.subtitle))}</div>`);
        }
        if (item?.body) {
          output.push(`<div class="jr-card-body">${escapeHtml(String(item.body))}</div>`);
        }
        output.push("</div>");
      });
      output.push("</div>");
      return output.join("");
    }
    case "gallery": {
      const items = Array.isArray(props.items) ? props.items : [];
      const output: string[] = [];
      output.push('<div class="jr-gallery">');
      items.forEach((item) => {
        output.push('<div class="jr-gallery-item">');
        const url = escapeAttribute(String(item?.url ?? ""));
        output.push(`<img class="jr-gallery-image" src="${url}" alt="" />`);
        output.push(`<div class="jr-gallery-caption">${escapeHtml(String(item?.caption ?? ""))}</div>`);
        output.push("</div>");
      });
      output.push("</div>");
      return output.join("");
    }
    case "table": {
      const rows = Array.isArray(props.rows) ? props.rows : [];
      const headers = normalizeTableHeaders(props.headers, rows);
      const tableRows = normalizeTableRows(rows, headers);
      if (tableRows.length === 0) {
        return renderFallback("Table is empty.");
      }
      const output: string[] = [];
      output.push('<div class="jr-table-wrap"><table class="jr-table">');
      output.push("<thead><tr>");
      headers.forEach((header) => {
        output.push(`<th>${escapeHtml(header)}</th>`);
      });
      output.push("</tr></thead><tbody>");
      tableRows.forEach((row) => {
        output.push("<tr>");
        row.forEach((cell) => {
          output.push(`<td>${escapeHtml(cell)}</td>`);
        });
        output.push("</tr>");
      });
      output.push("</tbody></table></div>");
      return output.join("");
    }
    case "kanban": {
      const columns = Array.isArray(props.columns) ? props.columns : [];
      if (columns.length === 0) {
        return renderFallback("Kanban has no columns.");
      }
      const output: string[] = [];
      output.push('<div class="jr-kanban">');
      columns.forEach((column, index) => {
        const title = escapeHtml(String(column?.title ?? `Column ${index + 1}`));
        output.push('<div class="jr-kanban-column">');
        output.push(`<div class="jr-kanban-title">${title}</div>`);
        const items = normalizeKanbanItems(column?.items);
        output.push('<div class="jr-kanban-items">');
        items.forEach((item) => {
          output.push('<div class="jr-kanban-card">');
          output.push(`<div class="jr-kanban-card-title">${escapeHtml(item.title)}</div>`);
          if (item.subtitle) {
            output.push(`<div class="jr-kanban-card-subtitle">${escapeHtml(item.subtitle)}</div>`);
          }
          if (item.meta) {
            output.push(`<div class="jr-kanban-card-meta">${escapeHtml(item.meta)}</div>`);
          }
          output.push("</div>");
        });
        output.push("</div></div>");
      });
      output.push("</div>");
      return output.join("");
    }
    case "diagram": {
      const nodes = Array.isArray(props.nodes) ? props.nodes : [];
      const edges = Array.isArray(props.edges) ? props.edges : [];
      if (nodes.length === 0 && edges.length === 0) {
        return renderFallback("Diagram has no nodes or edges.");
      }
      const output: string[] = [];
      output.push('<div class="jr-diagram">');
      if (nodes.length > 0) {
        output.push('<div class="jr-diagram-nodes">');
        nodes.forEach((node, index) => {
          const label = escapeHtml(String(node?.label ?? node?.id ?? `Node ${index + 1}`));
          output.push(`<div class="jr-diagram-node">${label}</div>`);
        });
        output.push("</div>");
      }
      if (edges.length > 0) {
        output.push('<div class="jr-diagram-edges">');
        edges.forEach((edge) => {
          const from = escapeHtml(String(edge?.from ?? "?"));
          const to = escapeHtml(String(edge?.to ?? "?"));
          const label = edge?.label ? ` (${escapeHtml(String(edge.label))})` : "";
          output.push(`<div class="jr-diagram-edge">${from} -> ${to}${label}</div>`);
        });
        output.push("</div>");
      }
      output.push("</div>");
      return output.join("");
    }
    default:
      return renderFallback(`Unsupported element: ${element.type}`);
  }
}

function renderChildren(element: JsonRenderElement, tree: JsonRenderTree): string {
  const children = Array.isArray(element.children) ? element.children : [];
  return children
    .map((child) => {
      if (typeof child === "string") {
        const resolved = tree.elements[child];
        return resolved
          ? renderElement(resolved, tree)
          : renderFallback(`Missing element: ${child}`);
      }
      return renderElement(child, tree);
    })
    .join("");
}

function renderFlow(nodes: any[], edges: any[], direction: string): string {
  const order = nodes.length > 0 ? nodes : deriveNodesFromEdges(edges);
  const arrow = direction === "vertical" ? "v" : "->";
  const output: string[] = [];
  output.push(`<div class="jr-flow jr-flow-${direction}">`);
  order.forEach((node, idx) => {
    output.push(`<div class="jr-flow-node">${escapeHtml(String(node.label ?? node.id ?? ""))}</div>`);
    if (idx < order.length - 1) {
      const edge = findEdge(edges, node.id, order[idx + 1]?.id);
      const label = edge?.label ? ` ${edge.label}` : "";
      output.push(`<div class="jr-flow-arrow">${arrow}${escapeHtml(label)}</div>`);
    }
  });
  output.push("</div>");
  return output.join("");
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

function isPlainObject(value: unknown): value is Record<string, any> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function clampHeadingLevel(value: any): number {
  const level = typeof value === "number" ? value : parseInt(String(value ?? ""), 10);
  if (Number.isNaN(level)) {
    return 2;
  }
  return Math.min(4, Math.max(1, level));
}

function flexAlign(value: any, fallback: string): string {
  const align = typeof value === "string" ? value : fallback;
  switch (align) {
    case "center":
      return "align-items:center;";
    case "end":
      return "align-items:flex-end;";
    case "stretch":
      return "align-items:stretch;";
    default:
      return `align-items:${align};`;
  }
}

function flexJustify(value: any, fallback: string): string {
  const justify = typeof value === "string" ? value : fallback;
  switch (justify) {
    case "center":
      return "justify-content:center;";
    case "end":
      return "justify-content:flex-end;";
    case "between":
      return "justify-content:space-between;";
    default:
      return `justify-content:${justify};`;
  }
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function escapeAttribute(value: string): string {
  return escapeHtml(value);
}

function renderFallback(message: string): string {
  return `<div class="jr-fallback">${escapeHtml(message)}</div>`;
}

const JR_STYLES = `
.jr-root { padding: 16px; display: flex; flex-direction: column; gap: 16px; font-family: ui-sans-serif, system-ui, -apple-system, sans-serif; color: #0f172a; }
.jr-column, .jr-row, .jr-list { display: flex; gap: 12px; }
.jr-column, .jr-list { flex-direction: column; }
.jr-row { flex-wrap: wrap; }
.jr-card { border: 1px solid #e2e8f0; border-radius: 14px; padding: 12px; background: #ffffff; box-shadow: 0 1px 2px rgba(15, 23, 42, 0.05); }
.jr-heading { margin: 0; }
.jr-heading-1 { font-size: 24px; font-weight: 600; }
.jr-heading-2 { font-size: 20px; font-weight: 600; }
.jr-heading-3 { font-size: 18px; font-weight: 600; }
.jr-heading-4 { font-size: 16px; font-weight: 600; }
.jr-text { margin: 0; font-size: 14px; }
.jr-badge { display: inline-flex; align-items: center; border: 1px solid #e2e8f0; border-radius: 999px; padding: 2px 8px; font-size: 11px; font-weight: 600; color: #334155; background: #f8fafc; }
.jr-divider { border: none; border-top: 1px solid #e2e8f0; margin: 8px 0; }
.jr-image { max-width: 100%; border-radius: 12px; }
.jr-flow { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }
.jr-flow-vertical { flex-direction: column; align-items: flex-start; }
.jr-flow-node { border: 1px solid #e2e8f0; border-radius: 10px; padding: 6px 10px; background: #f8fafc; font-size: 13px; font-weight: 600; }
.jr-flow-arrow { font-size: 11px; color: #64748b; }
.jr-form { display: flex; flex-direction: column; gap: 10px; }
.jr-field-label { font-size: 12px; font-weight: 600; color: #0f172a; }
.jr-input, .jr-textarea { width: 100%; border: 1px solid #e2e8f0; border-radius: 10px; padding: 8px 10px; font-size: 13px; }
.jr-textarea { min-height: 90px; }
.jr-section-title { font-size: 15px; font-weight: 600; }
.jr-dashboard { display: flex; flex-direction: column; gap: 12px; }
.jr-metrics { display: flex; flex-wrap: wrap; gap: 12px; }
.jr-metric { border: 1px solid #e2e8f0; border-radius: 12px; padding: 10px 12px; background: #f8fafc; min-width: 140px; }
.jr-metric-label { font-size: 11px; color: #64748b; }
.jr-metric-value { font-size: 18px; font-weight: 600; }
.jr-metric-trend { font-size: 11px; color: #64748b; }
.jr-list-item { border: 1px solid #e2e8f0; border-radius: 10px; padding: 8px 10px; background: #f8fafc; }
.jr-list-title { font-size: 13px; font-weight: 600; }
.jr-list-meta { font-size: 11px; color: #64748b; }
.jr-card-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; }
.jr-card-title { font-size: 13px; font-weight: 600; }
.jr-card-subtitle { font-size: 11px; color: #64748b; }
.jr-card-body { margin-top: 6px; font-size: 13px; }
.jr-gallery { display: flex; flex-wrap: wrap; gap: 12px; }
.jr-gallery-item { width: 200px; display: flex; flex-direction: column; gap: 6px; }
.jr-gallery-image { width: 100%; height: 120px; object-fit: cover; border-radius: 10px; border: 1px solid #e2e8f0; }
.jr-gallery-caption { font-size: 11px; color: #64748b; }
.jr-table-wrap { overflow-x: auto; border: 1px solid #e2e8f0; border-radius: 12px; }
.jr-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.jr-table th { text-align: left; padding: 8px 10px; background: #f1f5f9; font-size: 11px; text-transform: uppercase; color: #64748b; }
.jr-table td { padding: 8px 10px; border-top: 1px solid #e2e8f0; }
.jr-kanban { display: flex; gap: 12px; overflow-x: auto; padding-bottom: 4px; }
.jr-kanban-column { min-width: 200px; border: 1px solid #e2e8f0; border-radius: 12px; background: #f8fafc; padding: 10px; }
.jr-kanban-title { font-size: 13px; font-weight: 600; margin-bottom: 8px; }
.jr-kanban-items { display: flex; flex-direction: column; gap: 8px; }
.jr-kanban-card { border: 1px solid #e2e8f0; border-radius: 10px; background: #ffffff; padding: 8px 10px; }
.jr-kanban-card-title { font-size: 12px; font-weight: 600; }
.jr-kanban-card-subtitle, .jr-kanban-card-meta { font-size: 11px; color: #64748b; }
.jr-diagram { display: flex; flex-direction: column; gap: 8px; }
.jr-diagram-nodes { display: flex; flex-wrap: wrap; gap: 8px; }
.jr-diagram-node { border: 1px solid #e2e8f0; border-radius: 10px; padding: 6px 10px; background: #f8fafc; font-size: 12px; font-weight: 600; }
.jr-diagram-edges { display: flex; flex-direction: column; gap: 4px; font-size: 11px; color: #64748b; }
.jr-fallback { border: 1px dashed #cbd5f5; border-radius: 10px; padding: 8px; font-size: 12px; color: #64748b; background: #f1f5f9; }
`;
