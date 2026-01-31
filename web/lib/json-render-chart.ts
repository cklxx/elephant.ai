export type ChartSpec = {
  title?: string;
  subtitle?: string;
  data?: { values?: any[] };
  encoding?: {
    x?: { field?: string; type?: string; title?: string };
    y?: { field?: string; type?: string; title?: string };
    color?: {
      field?: string;
      type?: string;
      title?: string;
      scale?: { domain?: any[]; range?: string[] };
    };
  };
  mark?: { type?: string; point?: boolean } | string;
  width?: number;
  height?: number;
};

export type ChartPadding = {
  top: number;
  right: number;
  bottom: number;
  left: number;
};

export type ChartTick = {
  value: number;
  label: string;
  pos: number;
};

export type ChartPoint = {
  x: number;
  y: number;
  xValue: number;
  yValue: number;
  xLabel: string;
  yLabel: string;
};

export type ChartSeries = {
  key: string;
  color: string;
  points: ChartPoint[];
  path: string;
};

export type ChartLayout = {
  title?: string;
  subtitle?: string;
  width: number;
  height: number;
  padding: ChartPadding;
  plotWidth: number;
  plotHeight: number;
  xAxis: { label?: string; ticks: ChartTick[] };
  yAxis: { label?: string; ticks: ChartTick[] };
  series: ChartSeries[];
  showPoints: boolean;
  showLine: boolean;
  pointRadius: number;
};

const DEFAULT_COLORS = [
  "#2563eb",
  "#f59e0b",
  "#16a34a",
  "#ef4444",
  "#8b5cf6",
  "#0ea5e9",
];

const DEFAULT_WIDTH = 560;
const DEFAULT_HEIGHT = 320;
const DEFAULT_PADDING: ChartPadding = { top: 28, right: 28, bottom: 36, left: 48 };

export function buildChartLayout(spec: ChartSpec): ChartLayout | null {
  const values = Array.isArray(spec?.data?.values) ? spec.data?.values ?? [] : [];
  if (values.length === 0) {
    return null;
  }

  const xField = spec.encoding?.x?.field ?? "x";
  const yField = spec.encoding?.y?.field ?? "y";
  const colorField = spec.encoding?.color?.field;
  const xType = spec.encoding?.x?.type ?? "";
  const yType = spec.encoding?.y?.type ?? "";

  const rawPoints = values.map((row: any, idx: number) => ({
    xRaw: row?.[xField],
    yRaw: row?.[yField],
    colorRaw: colorField ? row?.[colorField] : undefined,
    index: idx,
  }));

  const xValues = rawPoints.map((point) => toNumber(point.xRaw));
  const useIndex = xValues.some((value) => !Number.isFinite(value));
  const indexedLabels = rawPoints.map((point, idx) => {
    if (point.xRaw == null) {
      return String(idx + 1);
    }
    return String(point.xRaw);
  });

  const seriesMap = new Map<string, { label: string; color: string; points: ChartPoint[] }>();
  const colorScale = spec.encoding?.color?.scale;

  const resolvedXValues: number[] = [];
  const resolvedYValues: number[] = [];

  rawPoints.forEach((point) => {
    const xValue = useIndex ? point.index : toNumber(point.xRaw);
    const yValue = toNumber(point.yRaw);
    if (!Number.isFinite(xValue) || !Number.isFinite(yValue)) {
      return;
    }
    resolvedXValues.push(xValue);
    resolvedYValues.push(yValue);

    const seriesLabel = colorField ? String(point.colorRaw ?? "Series") : "Series";
    let series = seriesMap.get(seriesLabel);
    if (!series) {
      const color = resolveSeriesColor(seriesLabel, seriesMap.size, colorScale);
      series = { label: seriesLabel, color, points: [] };
      seriesMap.set(seriesLabel, series);
    }

    const xLabel = useIndex
      ? indexedLabels[point.index] ?? String(point.index + 1)
      : formatTickLabel(xValue, xType);
    const yLabel = formatTickLabel(yValue, yType);

    series.points.push({
      x: xValue,
      y: yValue,
      xValue,
      yValue,
      xLabel,
      yLabel,
    });
  });

  if (resolvedXValues.length === 0 || resolvedYValues.length === 0) {
    return null;
  }

  const width = clampDimension(spec.width, DEFAULT_WIDTH, 260, 1000);
  const height = clampDimension(spec.height, DEFAULT_HEIGHT, 200, 800);
  const padding = { ...DEFAULT_PADDING };
  const plotWidth = Math.max(1, width - padding.left - padding.right);
  const plotHeight = Math.max(1, height - padding.top - padding.bottom);

  let xMin = Math.min(...resolvedXValues);
  let xMax = Math.max(...resolvedXValues);
  let yMin = Math.min(...resolvedYValues);
  let yMax = Math.max(...resolvedYValues);

  if (xMin === xMax) {
    xMin -= 1;
    xMax += 1;
  }
  if (yMin === yMax) {
    yMin -= 1;
    yMax += 1;
  }

  const scaleX = (value: number) => ((value - xMin) / (xMax - xMin)) * plotWidth;
  const scaleY = (value: number) => ((value - yMin) / (yMax - yMin)) * plotHeight;

  const xTicks = buildTicks(xMin, xMax, 5).map((value) => ({
    value,
    label: useIndex ? labelForIndex(value, indexedLabels) : formatTickLabel(value, xType),
    pos: padding.left + scaleX(value),
  }));

  const yTicks = buildTicks(yMin, yMax, 5).map((value) => ({
    value,
    label: formatTickLabel(value, yType),
    pos: padding.top + (plotHeight - scaleY(value)),
  }));

  const series: ChartSeries[] = [];
  for (const entry of seriesMap.values()) {
    const points = entry.points
      .slice()
      .sort((a, b) => a.xValue - b.xValue)
      .map((point) => ({
        ...point,
        x: padding.left + scaleX(point.xValue),
        y: padding.top + (plotHeight - scaleY(point.yValue)),
      }));
    const path = buildLinePath(points);
    series.push({
      key: entry.label,
      color: entry.color,
      points,
      path,
    });
  }

  const mark = spec.mark;
  const markType = typeof mark === "string" ? mark : mark?.type ?? "line";
  const showLine = markType === "line";
  const showPoints =
    markType === "point" || (typeof mark === "object" ? Boolean(mark?.point) : false);

  return {
    title: spec.title,
    subtitle: spec.subtitle,
    width,
    height,
    padding,
    plotWidth,
    plotHeight,
    xAxis: { label: spec.encoding?.x?.title, ticks: xTicks },
    yAxis: { label: spec.encoding?.y?.title, ticks: yTicks },
    series,
    showPoints,
    showLine,
    pointRadius: 3,
  };
}

function resolveSeriesColor(
  label: string,
  index: number,
  scale: { domain?: any[]; range?: string[] } | undefined,
): string {
  const domain = Array.isArray(scale?.domain) ? scale?.domain : [];
  const range = Array.isArray(scale?.range) ? scale?.range : [];
  if (domain.length > 0 && range.length > 0) {
    const domainIndex = domain.findIndex(
      (entry) => String(entry) === String(label),
    );
    if (domainIndex >= 0 && range[domainIndex]) {
      return String(range[domainIndex]);
    }
  }
  return DEFAULT_COLORS[index % DEFAULT_COLORS.length];
}

function buildLinePath(points: ChartPoint[]): string {
  if (points.length === 0) {
    return "";
  }
  return points
    .map((point, idx) => {
      const command = idx === 0 ? "M" : "L";
      return `${command}${point.x.toFixed(2)},${point.y.toFixed(2)}`;
    })
    .join(" ");
}

function buildTicks(min: number, max: number, count: number): number[] {
  if (!Number.isFinite(min) || !Number.isFinite(max)) {
    return [];
  }
  if (count <= 1) {
    return [min];
  }
  const step = (max - min) / (count - 1);
  return Array.from({ length: count }, (_, idx) => min + step * idx);
}

function labelForIndex(value: number, labels: string[]): string {
  if (labels.length === 0) {
    return String(Math.round(value) + 1);
  }
  const index = Math.min(labels.length - 1, Math.max(0, Math.round(value)));
  return labels[index] ?? String(index + 1);
}

function formatTickLabel(value: number, axisType: string): string {
  if (!Number.isFinite(value)) {
    return "";
  }
  if (axisType === "temporal") {
    if (value > 1e11) {
      const date = new Date(value);
      return String(date.getFullYear());
    }
    if (Math.abs(value) >= 1000) {
      return String(Math.round(value));
    }
  }
  if (Math.abs(value - Math.round(value)) < 1e-6) {
    return String(Math.round(value));
  }
  return value.toFixed(2).replace(/\.00$/, "");
}

function toNumber(value: any): number {
  if (typeof value === "number") {
    return value;
  }
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
    const date = Date.parse(value);
    if (!Number.isNaN(date)) {
      return date;
    }
  }
  return Number.NaN;
}

function clampDimension(
  value: any,
  fallback: number,
  min: number,
  max: number,
): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return Math.min(max, Math.max(min, value));
  }
  return fallback;
}
