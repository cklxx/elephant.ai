"use client";

import { useMemo, useState } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  FileText,
  Eye,
  Columns,
  Maximize2,
  Minimize2,
  BookOpen,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/lib/i18n";
import { Highlight, themes, Language } from "prism-react-renderer";
import { MarkdownRenderer } from "@/components/ui/markdown";
import {
  replacePlaceholdersWithMarkdown,
  buildAttachmentUri,
  getAttachmentSegmentType,
} from "@/lib/attachments";
import { AttachmentPayload } from "@/lib/types";
import { ImagePreview } from "@/components/ui/image-preview";
import { VideoPreview } from "@/components/ui/video-preview";
import { ArtifactPreviewCard } from "./ArtifactPreviewCard";

export type ViewMode = "default" | "reading" | "compare";

export interface DocumentContent {
  id: string;
  title: string;
  content: string;
  type: "markdown" | "text" | "code";
  language?: string;
  timestamp?: number;
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload>;
}

interface DocumentCanvasProps {
  document: DocumentContent | null;
  compareDocument?: DocumentContent | null;
  className?: string;
  initialMode?: ViewMode;
}

export function DocumentCanvas({
  document,
  compareDocument,
  className,
  initialMode = "default",
}: DocumentCanvasProps) {
  const t = useTranslation();
  const [viewMode, setViewMode] = useState<ViewMode>(initialMode);
  const [isExpanded, setIsExpanded] = useState(false);
  const viewMaxHeight = isExpanded ? "calc(100vh - 220px)" : "70vh";
  const viewHeightClass = isExpanded
    ? "max-h-[calc(100vh-220px)]"
    : "max-h-[70vh]";

  if (!document) {
    return (
      <Card className={cn(className)}>
        <CardContent className="flex h-64 flex-col items-center justify-center text-muted-foreground">
          <FileText className="h-16 w-16 mb-4 text-gray-300" />
          <p className="font-medium">{t("document.empty.title")}</p>
          <p className="text-sm mt-1">{t("document.empty.description")}</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div
      className={cn(
        "transition-all duration-300",
        isExpanded && "fixed inset-0 z-50 bg-background p-6 overflow-auto",
        className,
      )}
    >
      <Card className="h-full">
        {/* Header */}
        <div className="border-b border-border p-4">
          <div className="mb-3 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="rounded-lg border border-border bg-muted p-2">
                <FileText className="h-5 w-5 text-foreground" />
              </div>
              <div>
                <h3 className="font-semibold text-foreground">{document.title}</h3>
                {document.timestamp && (
                  <p className="text-xs text-muted-foreground">
                    {new Date(document.timestamp).toLocaleString()}
                  </p>
                )}
              </div>
            </div>

            <div className="flex items-center gap-2">
              {/* Expand toggle */}
              <Button
                onClick={() => setIsExpanded(!isExpanded)}
                variant="outline"
                size="sm"
                className="h-8 w-8 p-0"
              >
                {isExpanded ? (
                  <Minimize2 className="h-4 w-4" />
                ) : (
                  <Maximize2 className="h-4 w-4" />
                )}
              </Button>
            </div>
          </div>

          {/* View mode toggle */}
          <div className="flex flex-wrap items-center gap-2">
            <Button
              onClick={() => setViewMode("default")}
              variant={viewMode === "default" ? "default" : "outline"}
              size="sm"
              className="flex items-center gap-1.5"
            >
              <BookOpen className="h-3.5 w-3.5" />
              {t("document.view.default")}
            </Button>
            <Button
              onClick={() => setViewMode("reading")}
              variant={viewMode === "reading" ? "default" : "outline"}
              size="sm"
              className="flex items-center gap-1.5"
            >
              <Eye className="h-3.5 w-3.5" />
              {t("document.view.reading")}
            </Button>
            {compareDocument && (
              <Button
                onClick={() => setViewMode("compare")}
                variant={viewMode === "compare" ? "default" : "outline"}
                size="sm"
                className="flex items-center gap-1.5"
              >
                <Columns className="h-3.5 w-3.5" />
                {t("document.view.compare")}
              </Button>
            )}
          </div>

          {/* Metadata (hidden in reading mode) */}
          {viewMode !== "reading" && document.metadata && (
            <div className="mt-3 flex flex-wrap items-center gap-2">
              {Object.entries(document.metadata).map(([key, value]) => (
                <Badge key={key} variant="default" className="text-xs">
                  {key}: {String(value)}
                </Badge>
              ))}
            </div>
          )}
        </div>

        {/* Content */}
        <CardContent className="p-0">
          {viewMode === "compare" && compareDocument ? (
            <CompareView
              document={document}
              compareDocument={compareDocument}
              className={viewHeightClass}
              maxHeight={viewMaxHeight}
            />
          ) : viewMode === "reading" ? (
            <ReadingView document={document} className={viewHeightClass} />
          ) : (
            <DefaultView document={document} className={viewHeightClass} />
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function DefaultView({
  document,
  className,
}: {
  document: DocumentContent;
  className?: string;
}) {
  return (
    <div className={cn("overflow-auto", className)}>
      <DocumentRenderer document={document} showLineNumbers />
    </div>
  );
}

function ReadingView({
  document,
  className,
}: {
  document: DocumentContent;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "overflow-auto bg-gradient-to-br from-amber-50/30 to-white",
        className,
      )}
    >
      <div className="max-w-4xl mx-auto p-8">
        <DocumentRenderer document={document} cleanMode />
      </div>
    </div>
  );
}

function CompareView({
  document,
  compareDocument,
  className,
  maxHeight,
}: {
  document: DocumentContent;
  compareDocument: DocumentContent;
  className?: string;
  maxHeight?: string;
}) {
  const heightStyle = maxHeight
    ? { maxHeight, height: maxHeight }
    : undefined;

  return (
    <div
      className={cn(
        "grid grid-cols-1 grid-rows-[minmax(0,1fr)] divide-y divide-gray-200 overflow-hidden lg:grid-cols-2 lg:divide-y-0 lg:divide-x",
        className,
      )}
      style={heightStyle}
    >
      {/* Left pane */}
      <div className="flex min-h-0 flex-col overflow-auto">
        <div className="sticky top-0 bg-destructive/10 border-b border-destructive/30 px-4 py-2 z-10">
          <p className="text-sm font-semibold text-destructive">{document.title}</p>
        </div>
        <div className="p-4">
          <DocumentRenderer document={document} />
        </div>
      </div>

      {/* Right pane */}
      <div className="flex min-h-0 flex-col overflow-auto">
        <div className="sticky top-0 bg-emerald-50 border-b border-emerald-200 px-4 py-2 z-10">
          <p className="text-sm font-semibold text-emerald-700">{compareDocument.title}</p>
        </div>
        <div className="p-4">
          <DocumentRenderer document={compareDocument} />
        </div>
      </div>
    </div>
  );
}

function DocumentRenderer({
  document,
  showLineNumbers = false,
  cleanMode = false,
}: {
  document: DocumentContent;
  showLineNumbers?: boolean;
  cleanMode?: boolean;
}) {
  const typographyClass = cn(
    "prose max-w-none text-foreground",
    cleanMode ? "prose-lg leading-relaxed" : "prose-sm",
  );

  const containerClass = cn(!cleanMode && "px-4 py-3");

  if (document.type === "markdown") {
    const renderedContent = document.attachments
      ? replacePlaceholdersWithMarkdown(document.content, document.attachments)
      : document.content;

    return (
      <div className={containerClass}>
        <MarkdownRenderer
          content={renderedContent}
          className={typographyClass}
          attachments={document.attachments}
          showLineNumbers={showLineNumbers}
        />
        {document.attachments && Object.keys(document.attachments).length > 0 && (
          <AttachmentGallery attachments={document.attachments} />
        )}
      </div>
    );
  }

  if (document.type === "code") {
    return (
      <Highlight
        theme={themes.vsDark}
        code={document.content}
        language={(document.language || "text") as Language}
      >
        {({ className, style, tokens, getLineProps, getTokenProps }) => (
          <pre
            className={cn(className, "p-4 overflow-auto h-full")}
            style={style}
          >
            {tokens.map((line, i) => (
              <div key={i} {...getLineProps({ line })}>
                {showLineNumbers && !cleanMode && (
                  <span className="inline-block w-12 text-gray-500 select-none text-right pr-4">
                    {i + 1}
                  </span>
                )}
                {line.map((token, key) => (
                  <span key={key} {...getTokenProps({ token })} />
                ))}
              </div>
            ))}
          </pre>
        )}
      </Highlight>
    );
  }

  // Plain text
  return (
    <div className={cn("font-mono text-sm", containerClass)}>
      <pre className="whitespace-pre-wrap">{document.content}</pre>
    </div>
  );
}

interface AttachmentGalleryProps {
  attachments: Record<string, AttachmentPayload>;
}

type AttachmentKindFilter = "all" | "artifact" | "attachment";

interface NormalizedAttachment {
  key: string;
  attachment: AttachmentPayload;
  type: string;
  kind: string;
  formatValue: string | null;
  formatLabel: string | null;
}

function AttachmentGallery({ attachments }: AttachmentGalleryProps) {
  const t = useTranslation();
  const [kindFilter, setKindFilter] = useState<AttachmentKindFilter>("all");
  const [formatFilter, setFormatFilter] = useState<string>("all");
  const [searchQuery, setSearchQuery] = useState("");

  const normalized = useMemo<NormalizedAttachment[]>(() => {
    return Object.entries(attachments).map(([key, attachment]) => {
      const kind = attachment.kind ?? "attachment";
      const formatLabel = attachment.format ?? null;
      const formatValue = formatLabel ? formatLabel.toLowerCase() : null;
      return {
        key,
        attachment,
        type: getAttachmentSegmentType(attachment),
        kind,
        formatValue,
        formatLabel,
      };
    });
  }, [attachments]);

  const availableFormats = useMemo(() => {
    const seen = new Map<string, string>();
    normalized.forEach(({ formatValue, formatLabel }) => {
      if (!formatValue || seen.has(formatValue)) {
        return;
      }
      seen.set(formatValue, formatLabel ?? formatValue.toUpperCase());
    });
    return Array.from(seen.entries()).map(([value, label]) => ({ value, label }));
  }, [normalized]);

  const filtered = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    return normalized.filter(({ attachment, kind, formatValue, key }) => {
      const matchesKind =
        kindFilter === "all" ||
        (kindFilter === "artifact" ? kind === "artifact" : kind !== "artifact");
      if (!matchesKind) {
        return false;
      }

      const matchesFormat =
        formatFilter === "all" || (formatValue ? formatValue === formatFilter : false);
      if (!matchesFormat) {
        return false;
      }

      if (!query) {
        return true;
      }

      const haystacks = [
        key,
        attachment.name,
        attachment.description,
        attachment.media_type,
        attachment.format,
      ]
        .filter(Boolean)
        .map((value) => String(value).toLowerCase());

      return haystacks.some((value) => value.includes(query));
    });
  }, [normalized, kindFilter, formatFilter, searchQuery]);

  const grouped = filtered.reduce<Record<string, NormalizedAttachment[]>>((acc, item) => {
    if (!acc[item.type]) {
      acc[item.type] = [];
    }
    acc[item.type].push(item);
    return acc;
  }, {});

  const imageAttachments = grouped.image ?? [];
  const videoAttachments = grouped.video ?? [];
  const artifactAttachments = [...(grouped.document ?? []), ...(grouped.embed ?? [])];
  const hasMultipleArtifacts = artifactAttachments.length > 1;

  const kindOptions: { value: AttachmentKindFilter; label: string }[] = [
    {
      value: "all",
      label: t("document.attachments.filters.kind.all"),
    },
    {
      value: "attachment",
      label: t("document.attachments.filters.kind.attachments"),
    },
    {
      value: "artifact",
      label: t("document.attachments.filters.kind.artifacts"),
    },
  ];

  return (
    <div className="mt-6 space-y-4">
      <div className="space-y-3 rounded-xl border border-border/40 bg-muted/20 p-4">
        <div>
          <p className="text-[11px] font-semibold text-muted-foreground">
            {t("document.attachments.filters.heading")}
          </p>
          <div className="mt-2 flex flex-wrap gap-2">
            {kindOptions.map((option) => (
              <Button
                key={option.value}
                size="sm"
                variant={kindFilter === option.value ? "default" : "outline"}
                className="text-xs"
                onClick={() => setKindFilter(option.value)}
              >
                {option.label}
              </Button>
            ))}
          </div>
        </div>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          {availableFormats.length > 0 && (
            <label className="text-[11px] font-semibold text-muted-foreground">
              {t("document.attachments.filters.format.label")}
              <select
                className="mt-1 flex h-10 rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground transition placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:border-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50"
                value={formatFilter}
                onChange={(event) => setFormatFilter(event.target.value)}
              >
                <option value="all">
                  {t("document.attachments.filters.format.all")}
                </option>
                {availableFormats.map((item) => (
                  <option key={item.value} value={item.value}>
                    {item.label}
                  </option>
                ))}
              </select>
            </label>
          )}
          <div className="flex-1">
            <label className="text-[11px] font-semibold text-muted-foreground">
              {t("document.attachments.filters.search.label")}
              <input
                type="search"
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder={t("document.attachments.filters.search.placeholder")}
                className="mt-1 flex h-10 w-full rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground transition placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:border-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50"
              />
            </label>
          </div>
        </div>
      </div>
      {filtered.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border/60 bg-muted/10 p-6 text-center text-sm text-muted-foreground">
          {t("document.attachments.filters.empty")}
        </div>
      ) : (
        <>
          {imageAttachments.length > 0 && (
            <div
              className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-4"
              data-testid="document-attachment-images"
            >
              {imageAttachments.map(({ key, attachment }) => {
                const uri = buildAttachmentUri(attachment);
                if (!uri) {
                  return null;
                }
                return (
                  <ImagePreview
                    key={`doc-image-${key}`}
                    src={uri}
                    alt={attachment.description || attachment.name || key}
                    minHeight="12rem"
                    maxHeight="20rem"
                    sizes="(min-width: 1280px) 33vw, (min-width: 768px) 50vw, 100vw"
                  />
                );
              })}
            </div>
          )}
          {videoAttachments.length > 0 && (
            <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-4">
              {videoAttachments.map(({ key, attachment }) => {
                const uri = buildAttachmentUri(attachment);
                if (!uri) {
                  return null;
                }
                return (
                  <VideoPreview
                    key={`doc-video-${key}`}
                    src={uri}
                    mimeType={attachment.media_type || "video/mp4"}
                    description={attachment.description}
                    maxHeight="20rem"
                  />
                );
              })}
            </div>
          )}
          {artifactAttachments.length > 0 && (
            <div
              className={
                hasMultipleArtifacts
                  ? "grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-3"
                  : "space-y-3"
              }
            >
              {artifactAttachments.map(({ key, attachment }) => (
                <ArtifactPreviewCard
                  key={`doc-artifact-${key}`}
                  attachment={attachment}
                  displayMode={hasMultipleArtifacts ? "title" : undefined}
                />
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}
