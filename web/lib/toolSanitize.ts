export function sanitizeToolMetadataForUI(
  toolName: string,
  metadata?: Record<string, any> | null,
): Record<string, any> | null | undefined {
  if (!metadata || typeof metadata !== "object") {
    return metadata;
  }

  const normalized = toolName.toLowerCase().trim();
  if (normalized !== "plan") {
    return metadata;
  }

  const cleaned: Record<string, any> = { ...metadata };
  delete cleaned.internal_plan;
  delete cleaned.internalPlan;

  return Object.keys(cleaned).length > 0 ? cleaned : null;
}

