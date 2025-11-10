"use client";

import { useState } from "react";
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
import { replacePlaceholdersWithMarkdown, buildAttachmentUri } from "@/lib/attachments";
import { AttachmentPayload } from "@/lib/types";
import { ImagePreview } from "@/components/ui/image-preview";

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

  if (!document) {
    return (
      <Card className={cn("glass-card shadow-medium", className)}>
        <CardContent className="flex flex-col items-center justify-center h-64 text-gray-500">
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
        isExpanded && "fixed inset-0 z-50 bg-white p-6 overflow-auto",
        className,
      )}
    >
      <Card className="glass-card shadow-medium h-full">
        {/* Header */}
        <div className="border-b border-gray-200 p-4">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-gradient-to-br from-indigo-500 to-indigo-600 rounded-lg">
                <FileText className="h-5 w-5 text-white" />
              </div>
              <div>
                <h3 className="font-semibold text-gray-900">
                  {document.title}
                </h3>
                {document.timestamp && (
                  <p className="text-xs text-gray-500">
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
          <div className="flex items-center gap-2">
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
            <div className="flex items-center gap-2 flex-wrap mt-3">
              {Object.entries(document.metadata).map(([key, value]) => (
                <Badge key={key} variant="default" className="text-xs">
                  {key}: {String(value)}
                </Badge>
              ))}
            </div>
          )}
        </div>

        {/* Content */}
        <CardContent
          className={cn(
            "p-0",
            isExpanded ? "h-[calc(100%-100px)]" : "h-[600px]",
          )}
        >
          {viewMode === "compare" && compareDocument ? (
            <CompareView
              document={document}
              compareDocument={compareDocument}
            />
          ) : viewMode === "reading" ? (
            <ReadingView document={document} />
          ) : (
            <DefaultView document={document} />
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function DefaultView({ document }: { document: DocumentContent }) {
  return (
    <div className="h-full overflow-auto">
      <DocumentRenderer document={document} showLineNumbers />
    </div>
  );
}

function ReadingView({ document }: { document: DocumentContent }) {
  return (
    <div className="h-full overflow-auto bg-gradient-to-br from-amber-50/30 to-white">
      <div className="max-w-4xl mx-auto p-8">
        <DocumentRenderer document={document} cleanMode />
      </div>
    </div>
  );
}

function CompareView({
  document,
  compareDocument,
}: {
  document: DocumentContent;
  compareDocument: DocumentContent;
}) {
  return (
    <div className="h-full grid grid-cols-2 divide-x divide-gray-200">
      {/* Left pane */}
      <div className="overflow-auto">
        <div className="sticky top-0 bg-destructive/10 border-b border-destructive/30 px-4 py-2 z-10">
          <p className="text-sm font-semibold text-destructive">{document.title}</p>
        </div>
        <div className="p-4">
          <DocumentRenderer document={document} />
        </div>
      </div>

      {/* Right pane */}
      <div className="overflow-auto">
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
          showLineNumbers={showLineNumbers}
        />
        {document.attachments && Object.keys(document.attachments).length > 0 && (
          <div className="mt-6 grid gap-4 sm:grid-cols-2">
            {Object.entries(document.attachments).map(([key, attachment]) => {
              const uri = buildAttachmentUri(attachment);
              if (!uri) {
                return null;
              }
              return (
                <ImagePreview
                  key={key}
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
