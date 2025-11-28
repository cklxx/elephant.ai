"use client";

import { useState, useEffect } from "react";
import Image from "next/image";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Monitor,
  ChevronLeft,
  ChevronRight,
  Maximize2,
  X,
  FileText,
  Terminal,
  Globe,
  Code,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Highlight, themes } from "prism-react-renderer";
import { useTranslation } from "@/lib/i18n";
import { parseContentSegments, buildAttachmentUri } from "@/lib/attachments";
import { AttachmentPayload } from "@/lib/types";
import { ImagePreview } from "@/components/ui/image-preview";
import { VideoPreview } from "@/components/ui/video-preview";
import { ArtifactPreviewCard } from "./ArtifactPreviewCard";

export type ToolOutputType = 'web_fetch' | 'bash' | 'file_read' | 'file_write' | 'file_edit' | 'generic';

export interface ToolOutput {
  id: string;
  type: ToolOutputType;
  toolName: string;
  timestamp: number;
  // For web_fetch
  url?: string;
  screenshot?: string; // base64 encoded
  htmlPreview?: string;
  // For bash
  command?: string;
  stdout?: string;
  stderr?: string;
  exitCode?: number;
  // For file operations
  filePath?: string;
  content?: string;
  oldContent?: string; // for edits/diffs
  newContent?: string; // for edits/diffs
  // Generic
  result?: string;
  attachments?: Record<string, AttachmentPayload>;
}

interface WebViewportProps {
  outputs: ToolOutput[];
  className?: string;
}

export function WebViewport({ outputs, className }: WebViewportProps) {
  const t = useTranslation();
  const [currentIndex, setCurrentIndex] = useState(0);
  const [isFullscreen, setIsFullscreen] = useState(false);

  useEffect(() => {
    // Auto-advance to latest output
    if (outputs.length > 0) {
      setCurrentIndex(outputs.length - 1);
    }
  }, [outputs.length]);

  if (outputs.length === 0) {
    return (
      <Card className={cn(className)}>
        <CardContent className="flex h-64 flex-col items-center justify-center text-muted-foreground">
          <Monitor className="mb-4 h-16 w-16 text-muted-foreground/50" />
          <p className="font-medium text-foreground">{t("viewport.empty.title")}</p>
          <p className="mt-1 text-sm">{t("viewport.empty.description")}</p>
        </CardContent>
      </Card>
    );
  }

  const currentOutput = outputs[currentIndex];
  const hasPrevious = currentIndex > 0;
  const hasNext = currentIndex < outputs.length - 1;

  const handlePrevious = () => {
    if (hasPrevious) {
      setCurrentIndex(currentIndex - 1);
    }
  };

  const handleNext = () => {
    if (hasNext) {
      setCurrentIndex(currentIndex + 1);
    }
  };

  return (
    <>
      <Card className={cn(className)}>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="rounded-lg border border-border bg-muted px-2 py-2">
                <Monitor className="h-5 w-5 text-foreground" />
              </div>
              <div>
                <h3 className="font-semibold text-foreground">{t("viewport.title")}</h3>
                <p className="text-xs text-muted-foreground">
                  {t("viewport.position", { index: currentIndex + 1, total: outputs.length })}
                </p>
              </div>
            </div>

            <div className="flex items-center gap-2">
              {/* Carousel controls */}
              {outputs.length > 1 && (
                <>
                  <Button
                    onClick={handlePrevious}
                    disabled={!hasPrevious}
                    variant="outline"
                    size="sm"
                    className="h-8 w-8 p-0"
                    aria-label={t("viewport.aria.previous")}
                  >
                    <ChevronLeft className="h-4 w-4" />
                  </Button>
                  <Button
                    onClick={handleNext}
                    disabled={!hasNext}
                    variant="outline"
                    size="sm"
                    className="h-8 w-8 p-0"
                    aria-label={t("viewport.aria.next")}
                  >
                    <ChevronRight className="h-4 w-4" />
                  </Button>
                </>
              )}

              {/* Fullscreen toggle */}
              <Button
                onClick={() => setIsFullscreen(true)}
                variant="outline"
                size="sm"
                className="h-8 w-8 p-0"
                aria-label={t("viewport.aria.enterFullscreen")}
              >
                <Maximize2 className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </CardHeader>

        <CardContent>
          <OutputRenderer output={currentOutput} />
        </CardContent>
      </Card>

      {/* Fullscreen modal */}
      {isFullscreen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-background/98 text-foreground"
          role="dialog"
          aria-modal="true"
          aria-label={t("viewport.aria.fullscreenViewer")}
        >
          <div className="absolute top-4 right-4">
            <Button
              onClick={() => setIsFullscreen(false)}
              variant="outline"
              size="sm"
              aria-label={t("viewport.aria.exitFullscreen")}
            >
              <X className="h-4 w-4 mr-2" />
              {t("viewport.fullscreen.close")}
            </Button>
          </div>
          <div className="h-full w-full overflow-auto p-8">
            <OutputRenderer output={currentOutput} fullscreen />
          </div>
        </div>
      )}
    </>
  );
}

function OutputRenderer({
  output,
  fullscreen = false,
}: {
  output: ToolOutput;
  fullscreen?: boolean;
}) {
  const containerClass = fullscreen ? "text-foreground" : "text-foreground";

  return (
    <div className={cn("space-y-3", containerClass)}>
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border pb-2">
        <div className="flex items-center gap-2">
          <ToolTypeIcon type={output.type} />
          <span className="font-semibold">{output.toolName}</span>
        </div>
        <Badge variant="outline" className="text-xs font-mono">
          {new Date(output.timestamp).toLocaleTimeString()}
        </Badge>
      </div>

      {/* Content based on type */}
      {output.type === 'web_fetch' && (
        <WebFetchOutput
          url={output.url}
          screenshot={output.screenshot}
          htmlPreview={output.htmlPreview}
          fullscreen={fullscreen}
        />
      )}

      {output.type === 'bash' && (
        <BashOutput
          command={output.command}
          stdout={output.stdout}
          stderr={output.stderr}
          exitCode={output.exitCode}
          fullscreen={fullscreen}
        />
      )}

      {(output.type === 'file_read' || output.type === 'file_write') && (
        <FileOutput
          filePath={output.filePath}
          content={output.content}
          fullscreen={fullscreen}
        />
      )}

      {output.type === 'file_edit' && (
        <FileDiffOutput
          filePath={output.filePath}
          oldContent={output.oldContent}
          newContent={output.newContent}
          fullscreen={fullscreen}
        />
      )}

      {output.type === 'generic' && (
        <GenericOutput
          result={output.result}
          attachments={output.attachments}
          fullscreen={fullscreen}
        />
      )}
    </div>
  );
}

function ToolTypeIcon({ type }: { type: ToolOutputType }) {
  const iconClass = 'h-4 w-4';

  switch (type) {
    case 'web_fetch':
      return <Globe className={iconClass} />;
    case 'bash':
      return <Terminal className={iconClass} />;
    case 'file_read':
    case 'file_write':
    case 'file_edit':
      return <FileText className={iconClass} />;
    default:
      return <Code className={iconClass} />;
  }
}

function WebFetchOutput({
  url,
  screenshot,
  htmlPreview,
  fullscreen,
}: {
  url?: string;
  screenshot?: string;
  htmlPreview?: string;
  fullscreen: boolean;
}) {
  const t = useTranslation();
  const [showMode, setShowMode] = useState<'screenshot' | 'html'>('screenshot');

  return (
    <div className="space-y-3">
      {url && (
        <div className="text-sm">
          <span className="font-semibold text-gray-600">{t('viewport.web.url')}</span>
          <a
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            className="ml-2 text-primary hover:underline"
          >
            {url}
          </a>
        </div>
      )}

      {screenshot && htmlPreview && (
        <div className="flex gap-2 mb-2">
          <Button
            onClick={() => setShowMode('screenshot')}
            variant={showMode === 'screenshot' ? 'default' : 'outline'}
            size="sm"
          >
            {t('viewport.web.screenshot')}
          </Button>
          <Button
            onClick={() => setShowMode('html')}
            variant={showMode === 'html' ? 'default' : 'outline'}
            size="sm"
          >
            {t('viewport.web.html')}
          </Button>
        </div>
      )}

      {screenshot && showMode === 'screenshot' && (
        <div className="bg-white rounded-lg overflow-hidden border border-gray-200">
          <Image
            src={screenshot}
            alt={t('viewport.web.screenshotAlt')}
            width={1280}
            height={720}
            className={cn('h-auto w-full', fullscreen ? 'max-h-none' : 'max-h-96 object-contain')}
            unoptimized
            sizes="(max-width: 1024px) 100vw, 960px"
          />
        </div>
      )}

      {htmlPreview && showMode === 'html' && (
        <div
          className={cn(
            'bg-gray-50 border border-gray-200 rounded-lg p-4 overflow-auto font-mono text-xs',
            fullscreen ? 'max-h-none' : 'max-h-96'
          )}
        >
          <pre className="whitespace-pre-wrap">{htmlPreview}</pre>
        </div>
      )}
    </div>
  );
}

function BashOutput({
  command,
  stdout,
  stderr,
  exitCode,
  fullscreen,
}: {
  command?: string;
  stdout?: string;
  stderr?: string;
  exitCode?: number;
  fullscreen: boolean;
}) {
  const t = useTranslation();
  return (
    <div className="space-y-3">
      {command && (
        <div>
          <p className="text-xs font-semibold text-gray-600 mb-1">{t('viewport.bash.command')}</p>
          <div className="bg-gray-900 text-emerald-400 rounded-lg p-3 font-mono text-sm">
            <span className="text-gray-500">$</span> {command}
          </div>
        </div>
      )}

      {stdout && (
        <div>
          <p className="text-xs font-semibold text-gray-600 mb-1">{t('viewport.bash.output')}</p>
          <div
            className={cn(
              'bg-gray-900 text-gray-100 rounded-lg p-3 font-mono text-xs overflow-auto',
              fullscreen ? 'max-h-none' : 'max-h-64'
            )}
          >
            <pre className="whitespace-pre-wrap">{stdout}</pre>
          </div>
        </div>
      )}

      {stderr && (
        <div>
          <p className="text-xs font-semibold text-destructive mb-1">{t('viewport.bash.errorOutput')}</p>
          <div className="bg-destructive/10 text-destructive rounded-lg p-3 font-mono text-xs overflow-auto max-h-32">
            <pre className="whitespace-pre-wrap">{stderr}</pre>
          </div>
        </div>
      )}

      {exitCode !== undefined && (
        <div className="flex items-center gap-2">
          <span className="text-xs font-semibold text-gray-600">{t('viewport.bash.exitCode')}</span>
          <Badge variant={exitCode === 0 ? 'success' : 'error'}>{exitCode}</Badge>
        </div>
      )}
    </div>
  );
}

function FileOutput({
  filePath,
  content,
  fullscreen,
}: {
  filePath?: string;
  content?: string;
  fullscreen: boolean;
}) {
  return (
    <div className="space-y-3">
      {filePath && (
        <div className="text-sm font-mono text-gray-600 bg-gray-50 px-3 py-1.5 rounded border border-gray-200">
          {filePath}
        </div>
      )}

      {content && (
        <div
          className={cn(
            'bg-gray-900 rounded-lg overflow-auto',
            fullscreen ? 'max-h-none' : 'max-h-96'
          )}
        >
          <Highlight theme={themes.vsDark} code={content} language="tsx">
            {({ className, style, tokens, getLineProps, getTokenProps }) => (
              <pre className={cn(className, 'p-4 text-xs')} style={style}>
                {tokens.map((line, i) => (
                  <div key={i} {...getLineProps({ line })}>
                    <span className="inline-block w-8 text-gray-500 select-none">{i + 1}</span>
                    {line.map((token, key) => (
                      <span key={key} {...getTokenProps({ token })} />
                    ))}
                  </div>
                ))}
              </pre>
            )}
          </Highlight>
        </div>
      )}
    </div>
  );
}

function FileDiffOutput({
  filePath,
  oldContent,
  newContent,
  fullscreen,
}: {
  filePath?: string;
  oldContent?: string;
  newContent?: string;
  fullscreen: boolean;
}) {
  const t = useTranslation();
  const [viewMode, setViewMode] = useState<'split' | 'unified'>('split');

  return (
    <div className="space-y-3">
      {filePath && (
        <div className="text-sm font-mono text-gray-600 bg-gray-50 px-3 py-1.5 rounded border border-gray-200">
          {filePath}
        </div>
      )}

      <div className="flex gap-2">
        <Button
          onClick={() => setViewMode('split')}
          variant={viewMode === 'split' ? 'default' : 'outline'}
          size="sm"
        >
          {t('viewport.diff.split')}
        </Button>
        <Button
          onClick={() => setViewMode('unified')}
          variant={viewMode === 'unified' ? 'default' : 'outline'}
          size="sm"
        >
          {t('viewport.diff.unified')}
        </Button>
      </div>

      {viewMode === 'split' ? (
        <div className="grid grid-cols-2 gap-2">
          <div>
            <p className="text-xs font-semibold text-destructive mb-1 px-2">{t('viewport.diff.before')}</p>
            <div className="bg-destructive/10 border border-destructive/30 rounded-lg overflow-auto max-h-96">
              <pre className="p-3 text-xs font-mono whitespace-pre-wrap">{oldContent}</pre>
            </div>
          </div>
          <div>
            <p className="text-xs font-semibold text-emerald-600 mb-1 px-2">{t('viewport.diff.after')}</p>
            <div className="bg-emerald-50 border border-emerald-200 rounded-lg overflow-auto max-h-96">
              <pre className="p-3 text-xs font-mono whitespace-pre-wrap">{newContent}</pre>
            </div>
          </div>
        </div>
      ) : (
        <div className="bg-gray-900 text-gray-100 rounded-lg overflow-auto max-h-96 p-3 font-mono text-xs">
          <div className="text-destructive">
            <div className="text-gray-500">{t('viewport.diff.beforeHeading')}</div>
            <pre className="whitespace-pre-wrap">{oldContent}</pre>
          </div>
          <div className="text-emerald-400 mt-2">
            <div className="text-gray-500">{t('viewport.diff.afterHeading')}</div>
            <pre className="whitespace-pre-wrap">{newContent}</pre>
          </div>
        </div>
      )}
    </div>
  );
}

function GenericOutput({
  result,
  attachments,
  fullscreen,
}: {
  result?: string;
  attachments?: Record<string, AttachmentPayload>;
  fullscreen: boolean;
}) {
  const segments = parseContentSegments(result ?? '', attachments);
  const textSegments = segments.filter(
    (segment) => segment.type === 'text' && segment.text && segment.text.length > 0,
  );
  const mediaSegments = segments.filter(
    (segment) => segment.type === 'image' || segment.type === 'video',
  );
  const artifactSegments = segments.filter(
    (segment) =>
      (segment.type === 'document' || segment.type === 'embed') && segment.attachment,
  );
  return (
    <div
      className={cn(
        'bg-gray-50 border border-gray-200 rounded-lg p-4 overflow-auto font-mono text-xs space-y-3',
        fullscreen ? 'max-h-none' : 'max-h-96'
      )}
    >
      {textSegments.map((segment, index) => (
        <span key={`text-segment-${index}`}>{segment.text}</span>
      ))}
      {mediaSegments.length > 0 && (
        <div className="grid gap-4">
          {mediaSegments.map((segment, index) => {
            if (!segment.attachment) {
              return null;
            }
            const uri = buildAttachmentUri(segment.attachment);
            if (!uri) {
              return null;
            }
            const key = segment.placeholder || `${segment.type}-${index}`;
            if (segment.type === 'video') {
              return (
                <VideoPreview
                  key={`generic-output-media-${key}`}
                  src={uri}
                  mimeType={segment.attachment.media_type || 'video/mp4'}
                  description={segment.attachment.description}
                  maxHeight={fullscreen ? "28rem" : "18rem"}
                />
              );
            }
            return (
              <ImagePreview
                key={`generic-output-media-${key}`}
                src={uri}
                alt={segment.attachment.description || segment.attachment.name}
                minHeight={fullscreen ? "18rem" : "12rem"}
                maxHeight={fullscreen ? "28rem" : "18rem"}
                sizes="(min-width: 1280px) 50vw, (min-width: 768px) 66vw, 100vw"
              />
            );
          })}
        </div>
      )}
      {artifactSegments.length > 0 && (
        <div className="space-y-3">
          {artifactSegments.map((segment, index) => {
            if (!segment.attachment) {
              return null;
            }
            const key = segment.placeholder || `artifact-${index}`;
            return (
              <ArtifactPreviewCard
                key={`generic-output-artifact-${key}`}
                attachment={segment.attachment}
                className={fullscreen ? undefined : 'bg-white/80'}
              />
            );
          })}
        </div>
      )}
    </div>
  );
}
