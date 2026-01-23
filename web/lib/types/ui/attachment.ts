export interface AttachmentPreviewAssetPayload {
  asset_id?: string;
  label?: string;
  mime_type?: string;
  cdn_url?: string;
  preview_type?: string;
}

export interface AttachmentPayload {
  name: string;
  media_type: string;
  data?: string;
  uri?: string;
  source?: string;
  description?: string;
  kind?: 'attachment' | 'artifact' | string;
  format?: string;
  preview_profile?: string;
  preview_assets?: AttachmentPreviewAssetPayload[];
  visibility?: 'default' | 'recalled' | string;
  retention_ttl_seconds?: number;
  size?: number;
}

export interface AttachmentUpload {
  name: string;
  media_type: string;
  data?: string;
  uri?: string;
  source?: string;
  description?: string;
  kind?: 'attachment' | 'artifact' | string;
  format?: string;
  retention_ttl_seconds?: number;
}
