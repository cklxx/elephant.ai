import { A2UIAttachmentPreview } from "@/components/agent/A2UIAttachmentPreview";
import { ArtifactPreviewCard } from "@/components/agent/ArtifactPreviewCard";
import { ImagePreview } from "@/components/ui/image-preview";
import { VideoPreview } from "@/components/ui/video-preview";
import { buildAttachmentUri, type ContentSegment } from "@/lib/attachments";
import type { AttachmentPayload } from "@/lib/types";
import { cn } from "@/lib/utils";

interface TaskCompleteAttachmentsProps {
  streamInProgress: boolean;
  a2uiAttachments: Record<string, AttachmentPayload>;
  unreferencedMediaSegments: ContentSegment[];
  artifactSegments: ContentSegment[];
  hasMultipleArtifacts: boolean;
}

export function TaskCompleteAttachments({
  streamInProgress,
  a2uiAttachments,
  unreferencedMediaSegments,
  artifactSegments,
  hasMultipleArtifacts,
}: TaskCompleteAttachmentsProps) {
  const hasA2UIAttachments = Object.keys(a2uiAttachments).length > 0;

  return (
    <>
      {!streamInProgress && hasA2UIAttachments && (
        <div className="mt-4 space-y-4">
          {Object.entries(a2uiAttachments).map(([key, attachment]) => (
            <A2UIAttachmentPreview
              key={`task-complete-a2ui-${key}`}
              attachment={attachment}
            />
          ))}
        </div>
      )}

      {!streamInProgress && unreferencedMediaSegments.length > 0 && (
        <div className="flex flex-wrap items-start gap-3">
          {unreferencedMediaSegments.map((segment, index) => {
            if (!segment.attachment) {
              return null;
            }
            const uri = buildAttachmentUri(segment.attachment);
            if (!uri) {
              return null;
            }
            const caption =
              segment.attachment.description ||
              segment.attachment.name ||
              `image-${index + 1}`;
            const key = segment.placeholder || `${segment.type}-${index}`;
            if (segment.type === "video") {
              return (
                <VideoPreview
                  key={`task-complete-media-${key}`}
                  src={uri}
                  mimeType={segment.attachment.media_type || "video/mp4"}
                  description={segment.attachment.description}
                  className="w-full sm:w-[220px] lg:w-[260px]"
                  maxHeight="20rem"
                />
              );
            }
            return (
              <ImagePreview
                key={`task-complete-media-${key}`}
                src={uri}
                alt={caption}
                minHeight="12rem"
                maxHeight="20rem"
                className="w-full sm:w-[220px] lg:w-[260px]"
                sizes="(min-width: 1280px) 260px, (min-width: 768px) 220px, 100vw"
                loading={index === 0 ? "eager" : "lazy"}
              />
            );
          })}
        </div>
      )}

      {!streamInProgress && artifactSegments.length > 0 && (
        <div
          className={cn(
            "mt-4",
            hasMultipleArtifacts
              ? "grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-3"
              : "space-y-3",
          )}
        >
          {artifactSegments.map((segment, index) => {
            if (!segment.attachment) {
              return null;
            }
            const key = segment.placeholder || `artifact-${index}`;
            return (
              <ArtifactPreviewCard
                key={`task-complete-artifact-${key}`}
                attachment={segment.attachment}
                displayMode={hasMultipleArtifacts ? "title" : undefined}
              />
            );
          })}
        </div>
      )}
    </>
  );
}
