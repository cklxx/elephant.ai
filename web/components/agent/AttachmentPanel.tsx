"use client";

import { ReactNode, useMemo } from "react";
import { FileText, Images } from "lucide-react";

import {
  AttachmentSegmentType,
  buildAttachmentUri,
  getAttachmentSegmentType,
} from "@/lib/attachments";
import { AnyAgentEvent, AttachmentPayload, eventMatches } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { ImagePreview } from "@/components/ui/image-preview";
import { VideoPreview } from "@/components/ui/video-preview";
import { ArtifactPreviewCard } from "./ArtifactPreviewCard";

interface AttachmentPanelProps {
  events: AnyAgentEvent[];
}

export interface AttachmentListItem {
  key: string;
  attachment: AttachmentPayload;
  type: AttachmentSegmentType;
  source: string;
}

export function AttachmentPanel({ events }: AttachmentPanelProps) {
  const attachments = useMemo(() => collectAttachmentItems(events), [events]);
  const hasAttachments = attachments.length > 0;
  const hasMultipleAttachments = attachments.length > 1;
  const hasMultipleArtifacts =
    attachments.filter(
      (item) => item.type === "document" || item.type === "embed",
    ).length > 1;

  if (!hasAttachments) {
    return null;
  }

  return (
    <Card className="h-full rounded-2xl border border-border/60 bg-card">
      <CardHeader className="flex flex-row items-start justify-between gap-3 pb-4">
        <div className="space-y-1">
          <p className="text-sm font-semibold text-foreground">Attachments</p>
          <p className="text-xs text-muted-foreground">
            Tool outputs and generated artifacts for quick review.
          </p>
        </div>
        <Badge variant="outline" className="rounded-full text-[11px]">
          {attachments.length}
        </Badge>
      </CardHeader>
      <CardContent className="pt-0">
        <ScrollArea className="max-h-[70vh]">
          <div
            className={
              hasMultipleAttachments
                ? "grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-3 pr-1"
                : "flex flex-col gap-3 pr-1"
            }
          >
            {attachments.map((item) => (
              <AttachmentPreview
                key={item.key}
                item={item}
                compactDocuments={hasMultipleArtifacts}
              />
            ))}
          </div>
        </ScrollArea>
      </CardContent>
    </Card>
  );
}

function AttachmentPreview({
  item,
  compactDocuments,
}: {
  item: AttachmentListItem;
  compactDocuments: boolean;
}) {
  const uri = buildAttachmentUri(item.attachment);
  const title = item.attachment.description || item.attachment.name || item.key;
  const badgeLabel = formatTypeLabel(item.type, item.attachment);

  return (
    <div className="space-y-3 rounded-2xl border border-border/70 bg-background p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <p className="truncate text-sm font-semibold text-foreground">
            {title}
          </p>
          <p className="text-[11px] text-muted-foreground">{item.source}</p>
        </div>
        <Badge variant="outline" className="shrink-0 text-[11px] capitalize">
          {badgeLabel}
        </Badge>
      </div>
      <AttachmentBody
        type={item.type}
        attachment={item.attachment}
        uri={uri}
        compactDocuments={compactDocuments}
      />
    </div>
  );
}

function AttachmentBody({
  type,
  attachment,
  uri,
  compactDocuments,
}: {
  type: AttachmentSegmentType;
  attachment: AttachmentPayload;
  uri: string | null;
  compactDocuments: boolean;
}) {
  if (type === "image") {
    if (!uri) {
      return <AttachmentFallback label="Image preview unavailable" />;
    }
    return (
      <ImagePreview
        src={uri}
        alt={attachment.description || attachment.name || "Attachment image"}
        minHeight="10rem"
        maxHeight="18rem"
        sizes="(min-width: 1280px) 18rem, (min-width: 768px) 16rem, 100vw"
        className="w-full overflow-hidden rounded-xl"
        imageClassName="object-cover"
      />
    );
  }

  if (type === "video") {
    if (!uri) {
      return <AttachmentFallback label="Video preview unavailable" />;
    }
    return (
      <VideoPreview
        src={uri}
        mimeType={attachment.media_type || "video/mp4"}
        description={attachment.description || attachment.name}
        maxHeight="18rem"
      />
    );
  }

  if (type === "document" || type === "embed") {
    return (
      <ArtifactPreviewCard
        attachment={attachment}
        displayMode={compactDocuments ? "title" : undefined}
      />
    );
  }

  return (
    <AttachmentFallback
      label={attachment.media_type || attachment.format || "Attachment"}
      icon={<FileText className="h-4 w-4 text-muted-foreground" />}
    />
  );
}

function AttachmentFallback({
  label,
  icon,
}: {
  label: string;
  icon?: ReactNode;
}) {
  return (
    <div className="flex items-center gap-2 rounded-xl border border-dashed border-border/70 bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
      {icon ?? <Images className="h-4 w-4" />}
      <span className="truncate">{label}</span>
    </div>
  );
}

export function collectAttachmentItems(
  events: AnyAgentEvent[],
): AttachmentListItem[] {
  const seenKeys = new Set<string>();
  const seenNames = new Set<string>();
  const renderedKeys = new Set<string>();
  const renderedNames = new Set<string>();
  const items: AttachmentListItem[] = [];

  events.forEach((event) => {
    if (!isRenderedInMainStream(event)) {
      return;
    }
    const attachments = (event as AnyAgentEvent & { attachments?: Record<string, AttachmentPayload> }).attachments;
    if (!attachments || typeof attachments !== "object") {
      return;
    }

    Object.entries(attachments).forEach(([rawKey, rawAttachment]) => {
      if (!rawAttachment) {
        return;
      }
      const normalizedKey =
        (rawKey || rawAttachment.name || "").trim() || "";
      if (normalizedKey) {
        renderedKeys.add(normalizedKey);
      }
      const normalizedName = (rawAttachment.name || rawAttachment.description || normalizedKey).trim();
      if (normalizedName) {
        renderedNames.add(normalizedName);
      }
    });
  });

  for (let idx = events.length - 1; idx >= 0; idx -= 1) {
    const event = events[idx];
    if (isRenderedInMainStream(event)) {
      continue;
    }
    const attachments = (event as AnyAgentEvent & { attachments?: Record<string, AttachmentPayload> }).attachments;
    if (!attachments || typeof attachments !== "object") {
      continue;
    }

    Object.entries(attachments).forEach(([rawKey, rawAttachment]) => {
      if (!rawAttachment) {
        return;
      }
      const normalizedKey =
        (rawKey || rawAttachment.name || "").trim() ||
        `attachment-${items.length + 1}`;
      const normalizedName = (rawAttachment.name || rawAttachment.description || normalizedKey).trim();
      if (renderedKeys.has(normalizedKey) || (normalizedName && renderedNames.has(normalizedName))) {
        return;
      }
      if (seenKeys.has(normalizedKey) || (normalizedName && seenNames.has(normalizedName))) {
        return;
      }

      const normalizedAttachment: AttachmentPayload = {
        ...rawAttachment,
        name: rawAttachment.name?.trim() || normalizedKey,
      };

      items.push({
        key: normalizedKey,
        attachment: normalizedAttachment,
        type: getAttachmentSegmentType(normalizedAttachment),
        source: describeSource(event),
      });
      seenKeys.add(normalizedKey);
      if (normalizedName) {
        seenNames.add(normalizedName);
      }
    });
  }

  return items.reverse();
}

function isRenderedInMainStream(event: AnyAgentEvent): boolean {
  if (
    event.event_type === "workflow.input.received" ||
    eventMatches(event, "workflow.result.final", "workflow.result.final") ||
    eventMatches(event, "workflow.result.cancelled", "workflow.result.cancelled") ||
    eventMatches(event, "workflow.node.failed")
  ) {
    return true;
  }
  return false;
}

function describeSource(event: AnyAgentEvent): string {
  if (event.event_type === "workflow.input.received") {
    return "User input";
  }
  if (eventMatches(event, "workflow.tool.completed", "workflow.tool.completed")) {
    const toolName =
      ("tool_name" in event && typeof (event as any).tool_name === "string"
        ? (event as any).tool_name
        : undefined) ||
      ("tool" in event && typeof (event as any).tool === "string"
        ? (event as any).tool
        : undefined);
    return toolName ? `Tool · ${toolName}` : "Tool output";
  }
  if (eventMatches(event, "workflow.result.final", "workflow.result.final")) {
    return "Final answer";
  }
  return "Agent event";
}

function formatTypeLabel(
  type: AttachmentSegmentType,
  attachment: AttachmentPayload,
): string {
  const formatLabel = attachment.format?.toUpperCase();
  if (formatLabel) {
    return formatLabel;
  }

  switch (type) {
    case "image":
      return "Image";
    case "video":
      return "Video";
    case "document":
    case "embed":
      return "Artifact";
    default:
      return "Attachment";
  }
}
