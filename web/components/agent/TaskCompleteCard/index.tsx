"use client";

import { useMemo } from "react";
import type { ComponentType, HTMLAttributes } from "react";

import { useTranslation } from "@/lib/i18n";
import {
  replacePlaceholdersWithMarkdown,
  stripAttachmentPlaceholders,
  isA2UIAttachment,
} from "@/lib/attachments";
import type { AttachmentPayload, WorkflowResultFinalEvent } from "@/lib/types";
import { Card } from "@/components/ui/card";

import { TaskCompleteContent } from "./TaskCompleteContent";
import { TaskCompleteAttachments } from "./TaskCompleteAttachments";
import { useInlineAttachmentMap } from "./hooks/useInlineAttachmentMap";
import { useTaskCompleteSegments } from "./hooks/useTaskCompleteSegments";

interface StopReasonCopy {
  title: string;
  body?: string;
}

interface TaskCompleteCardProps {
  event: WorkflowResultFinalEvent;
}

const INLINE_MEDIA_REGEX = /(data:image\/[^\s)]+|\/api\/data\/[A-Za-z0-9]+)/g;

type SummaryBlockProps = HTMLAttributes<HTMLDivElement>;

type SummaryComponents = Record<string, ComponentType<SummaryBlockProps>>;

function convertInlineMediaToMarkdown(text: string): string {
  if (!text) return text;
  let result = text;
  const matches = Array.from(text.matchAll(INLINE_MEDIA_REGEX));
  for (let i = matches.length - 1; i >= 0; i -= 1) {
    const m = matches[i];
    if (!m || m.index === undefined) continue;
    const start = m.index;
    const end = start + m[0].length;
    const before = text.slice(Math.max(0, start - 3), start);
    if (before.includes("![")) {
      continue;
    }
    result = `${result.slice(0, start)}![inline media](${m[0]})${result.slice(end)}`;
  }
  return result;
}

function getStopReasonCopy(
  stopReason: string | undefined,
  t: ReturnType<typeof useTranslation>,
): StopReasonCopy {
  if (!stopReason) {
    return {
      title: t("events.taskComplete.empty"),
    };
  }

  if (stopReason === "cancelled") {
    return {
      title: t("events.taskComplete.cancelled.title"),
      body: t("events.taskComplete.cancelled.body"),
    };
  }

  return {
    title: t("events.taskComplete.empty"),
    body: t("events.taskComplete.stopReason", { reason: stopReason }),
  };
}

export function TaskCompleteCard({ event }: TaskCompleteCardProps) {
  const t = useTranslation();
  const answer = event.final_answer ?? "";
  const markdownAnswer = convertInlineMediaToMarkdown(answer);
  const attachments = event.attachments ?? undefined;

  const { a2uiAttachments, standardAttachments } = useMemo(() => {
    if (!attachments) {
      return {
        a2uiAttachments: {} as Record<string, AttachmentPayload>,
        standardAttachments: undefined as
          | Record<string, AttachmentPayload>
          | undefined,
      };
    }
    const a2ui: Record<string, AttachmentPayload> = {};
    const standard: Record<string, AttachmentPayload> = {};
    Object.entries(attachments).forEach(([key, attachment]) => {
      if (isA2UIAttachment(attachment)) {
        a2ui[key] = attachment;
      } else {
        standard[key] = attachment;
      }
    });
    return {
      a2uiAttachments: a2ui,
      standardAttachments:
        Object.keys(standard).length > 0 ? standard : undefined,
    };
  }, [attachments]);

  const streamInProgress =
    event.stream_finished === false ||
    (event.is_streaming === true && event.stream_finished !== true);
  const streamFinished = event.stream_finished === true;
  const inlineAttachments = streamInProgress ? undefined : standardAttachments;
  const sanitizedAnswer = stripAttachmentPlaceholders(
    markdownAnswer,
    a2uiAttachments,
  );
  const contentWithInlineMedia = replacePlaceholdersWithMarkdown(
    sanitizedAnswer,
    inlineAttachments,
  );

  const { inlineAttachmentMap, attachmentNames, hasAttachments, inlineImageMap } =
    useInlineAttachmentMap({
      content: contentWithInlineMedia,
      attachments: inlineAttachments,
    });

  const {
    inlineRenderBlocks,
    unreferencedMediaSegments,
    artifactSegments,
    hasMultipleArtifacts,
    shouldSoftenSummary,
  } = useTaskCompleteSegments({
    markdownAnswer: sanitizedAnswer,
    attachments: standardAttachments,
  });

  const hasA2UIAttachments = Object.keys(a2uiAttachments).length > 0;
  const hasAnswerContent = contentWithInlineMedia.trim().length > 0;
  const shouldRenderMarkdown = hasAnswerContent || streamInProgress;
  const hasUnrenderedAttachments =
    unreferencedMediaSegments.length > 0 || artifactSegments.length > 0;
  const shouldShowFallback =
    !shouldRenderMarkdown &&
    !hasUnrenderedAttachments &&
    !hasAttachments &&
    !hasA2UIAttachments;
  const shouldShowAttachmentNotice =
    !shouldRenderMarkdown &&
    !hasUnrenderedAttachments &&
    hasAttachments &&
    !hasA2UIAttachments;
  const stopReasonCopy = getStopReasonCopy(event.stop_reason, t);

  const summaryComponents = useMemo<SummaryComponents>(
    () => ({
      h1: (props: SummaryBlockProps) => (
        <div className="mt-2 font-medium text-foreground" {...props} />
      ),
      h2: (props: SummaryBlockProps) => (
        <div className="mt-2 font-medium text-foreground" {...props} />
      ),
      h3: (props: SummaryBlockProps) => (
        <div className="mt-2 font-medium text-foreground" {...props} />
      ),
      h4: (props: SummaryBlockProps) => (
        <div className="mt-2 font-medium text-foreground/90" {...props} />
      ),
      h5: (props: SummaryBlockProps) => (
        <div className="mt-2 font-medium text-foreground/80" {...props} />
      ),
      h6: (props: SummaryBlockProps) => (
        <div className="mt-2 font-medium text-foreground/70" {...props} />
      ),
      ul: (props: SummaryBlockProps) => (
        <div className="my-2 space-y-1" {...props} />
      ),
      ol: (props: SummaryBlockProps) => (
        <div className="my-2 space-y-1" {...props} />
      ),
      li: (props: SummaryBlockProps) => (
        <div className="whitespace-pre-wrap text-foreground" {...props} />
      ),
      blockquote: (props: SummaryBlockProps) => (
        <div
          className="my-2 border-l-2 border-border/40 pl-3 text-foreground/80"
          {...props}
        />
      ),
      hr: () => <div className="my-2 h-px w-full bg-border/40" />,
      strong: (props: SummaryBlockProps) => (
        <strong className="font-semibold text-foreground" {...props} />
      ),
    }),
    [],
  );

  return (
    <Card
      data-testid="task-complete-event"
      className="border-0 bg-transparent shadow-none"
    >
      <TaskCompleteContent
        streamInProgress={streamInProgress}
        streamFinished={streamFinished}
        showStreamingPlaceholder={streamInProgress && !hasAnswerContent}
        shouldRenderMarkdown={shouldRenderMarkdown}
        contentWithInlineMedia={contentWithInlineMedia}
        inlineAttachments={inlineAttachments}
        inlineRenderBlocks={inlineRenderBlocks}
        shouldSoftenSummary={shouldSoftenSummary}
        summaryComponents={summaryComponents}
        inlineAttachmentMap={inlineAttachmentMap}
        inlineImageMap={inlineImageMap}
        stopReasonCopy={stopReasonCopy}
        shouldShowFallback={shouldShowFallback}
        shouldShowAttachmentNotice={shouldShowAttachmentNotice}
        attachmentNames={attachmentNames}
        emptyCopy={t("events.taskComplete.empty")}
        attachmentsAvailableCopy={t("events.taskComplete.attachmentsAvailable")}
        streamingTitle={t("events.taskComplete.streaming")}
        streamingHint={t("events.taskComplete.streamingHint")}
      />

      <TaskCompleteAttachments
        streamInProgress={streamInProgress}
        a2uiAttachments={a2uiAttachments}
        unreferencedMediaSegments={unreferencedMediaSegments}
        artifactSegments={artifactSegments}
        hasMultipleArtifacts={hasMultipleArtifacts}
      />
    </Card>
  );
}
