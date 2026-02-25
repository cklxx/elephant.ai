"use client";

import React from "react";

export const HighlightText = React.memo(function HighlightText({
  text,
  search,
}: {
  text: string;
  search: string;
}) {
  if (!text || !search) {
    return <>{text}</>;
  }

  const lowerText = text.toLowerCase();
  const lowerSearch = search.toLowerCase();
  const parts: Array<{ text: string; highlight: boolean }> = [];
  let cursor = 0;

  while (cursor < text.length) {
    const index = lowerText.indexOf(lowerSearch, cursor);
    if (index === -1) {
      parts.push({ text: text.slice(cursor), highlight: false });
      break;
    }
    if (index > cursor) {
      parts.push({ text: text.slice(cursor, index), highlight: false });
    }
    parts.push({ text: text.slice(index, index + search.length), highlight: true });
    cursor = index + search.length;
  }

  return (
    <>
      {parts.map((part, idx) =>
        part.highlight ? (
          <mark key={idx} className="rounded-sm bg-yellow-200 px-0.5">
            {part.text}
          </mark>
        ) : (
          <span key={idx}>{part.text}</span>
        ),
      )}
    </>
  );
});
