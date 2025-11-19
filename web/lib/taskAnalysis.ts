export function isFallbackActionName(actionName?: string | null) {
  if (!actionName) {
    return false;
  }

  const normalized = actionName.trim().toLowerCase();
  if (!normalized) {
    return false;
  }

  return normalized === 'processing request' || normalized.startsWith('working on ');
}
