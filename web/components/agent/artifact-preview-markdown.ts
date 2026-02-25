export const normalizeTitle = (value: string | null) =>
  value
    ?.replace(/^\uFEFF/, "")
    .replace(/\.[^.]+$/, "")
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]+/gu, " ")
    .trim() || null;

export const stripRedundantHeading = (
  markdown: string,
  normalizedTitle: string | null,
) => {
  if (!markdown.trim()) return markdown;

  const lines = markdown.replace(/^\uFEFF/, "").split(/\r?\n/);

  let index = 0;
  while (index < lines.length && !lines[index].trim()) {
    index += 1;
  }

  if (index >= lines.length) {
    return markdown.trimStart();
  }

  const headingMatch = lines[index].match(/^#{1,6}\s+(.+?)\s*#*\s*$/);
  const headingText = headingMatch ? normalizeTitle(headingMatch[1]) : null;

  if (headingText && normalizedTitle && headingText === normalizedTitle) {
    index += 1;
    while (index < lines.length && !lines[index].trim()) {
      index += 1;
    }
    return lines.slice(index).join("\n").trimStart();
  }

  return markdown.trimStart();
};
