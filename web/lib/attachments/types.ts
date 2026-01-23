import type { AttachmentPayload } from "@/lib/types";

export interface ContentSegment {
  type: "text" | "image" | "video" | "document" | "embed";
  text?: string;
  placeholder?: string;
  attachment?: AttachmentPayload;
  implicit?: boolean;
}

export type AttachmentSegmentType = ContentSegment["type"];
