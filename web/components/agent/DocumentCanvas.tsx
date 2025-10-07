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
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Highlight, themes, Language } from "prism-react-renderer";

export type ViewMode = "default" | "reading" | "compare";

export interface DocumentContent {
  id: string;
  title: string;
  content: string;
  type: "markdown" | "text" | "code";
  language?: string;
  timestamp?: number;
  metadata?: Record<string, any>;
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
  const [viewMode, setViewMode] = useState<ViewMode>(initialMode);
  const [isExpanded, setIsExpanded] = useState(false);

  if (!document) {
    return (
      <Card className={cn("glass-card shadow-medium", className)}>
        <CardContent className="flex flex-col items-center justify-center h-64 text-gray-500">
          <FileText className="h-16 w-16 mb-4 text-gray-300" />
          <p className="font-medium">No document selected</p>
          <p className="text-sm mt-1">Document content will appear here</p>
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
              Default
            </Button>
            <Button
              onClick={() => setViewMode("reading")}
              variant={viewMode === "reading" ? "default" : "outline"}
              size="sm"
              className="flex items-center gap-1.5"
            >
              <Eye className="h-3.5 w-3.5" />
              Reading
            </Button>
            {compareDocument && (
              <Button
                onClick={() => setViewMode("compare")}
                variant={viewMode === "compare" ? "default" : "outline"}
                size="sm"
                className="flex items-center gap-1.5"
              >
                <Columns className="h-3.5 w-3.5" />
                Compare
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
        <div className="sticky top-0 bg-red-50 border-b border-red-200 px-4 py-2 z-10">
          <p className="text-sm font-semibold text-red-800">{document.title}</p>
        </div>
        <div className="p-4">
          <DocumentRenderer document={document} />
        </div>
      </div>

      {/* Right pane */}
      <div className="overflow-auto">
        <div className="sticky top-0 bg-green-50 border-b border-green-200 px-4 py-2 z-10">
          <p className="text-sm font-semibold text-green-800">
            {compareDocument.title}
          </p>
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
  const containerClass = cn(
    "prose prose-sm max-w-none",
    cleanMode && "prose-lg leading-relaxed",
    !cleanMode && "px-4 py-3",
  );

  if (document.type === "markdown") {
    return (
      <div className={containerClass}>
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={{
            code({ node, className, children, ...props }: any) {
              const match = /language-(\w+)/.exec(className || "");
              const language = match ? match[1] : "text";
              const inline = !className; // Inline code doesn't have language class

              if (inline) {
                return (
                  <code
                    className="bg-gray-100 text-gray-900 px-1.5 py-0.5 rounded text-sm font-mono"
                    {...props}
                  >
                    {children}
                  </code>
                );
              }

              return (
                <Highlight
                  theme={themes.vsDark}
                  code={String(children).replace(/\n$/, "")}
                  language={language as Language}
                >
                  {({
                    className,
                    style,
                    tokens,
                    getLineProps,
                    getTokenProps,
                  }) => (
                    <pre
                      className={cn(className, "rounded-lg overflow-auto")}
                      style={style}
                    >
                      {tokens.map((line, i) => (
                        <div key={i} {...getLineProps({ line })}>
                          {showLineNumbers && (
                            <span className="inline-block w-8 text-gray-500 select-none text-right pr-3">
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
            },
          }}
        >
          {document.content}
        </ReactMarkdown>
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
