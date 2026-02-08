"use client";

import { useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

export function VirtualizedList<T>({
  items,
  estimateSize,
  overscan = 8,
  className,
  contentClassName,
  itemKey,
  renderItem,
  empty,
}: {
  items: T[];
  estimateSize: number;
  overscan?: number;
  className?: string;
  contentClassName?: string;
  itemKey?: (item: T, index: number) => string;
  renderItem: (item: T, index: number) => React.ReactNode;
  empty?: React.ReactNode;
}) {
  const parentRef = useRef<HTMLDivElement>(null);

  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => estimateSize,
    overscan,
  });

  return (
    <div ref={parentRef} className={className}>
      {items.length === 0 ? (
        empty ?? null
      ) : (
        <div
          className={contentClassName}
          style={{
            height: virtualizer.getTotalSize(),
            position: "relative",
          }}
        >
          {virtualizer.getVirtualItems().map((virtualRow) => {
            const item = items[virtualRow.index];
            return (
              <div
                key={itemKey ? itemKey(item, virtualRow.index) : virtualRow.key}
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  transform: `translateY(${virtualRow.start}px)`,
                }}
                ref={virtualizer.measureElement}
                data-index={virtualRow.index}
              >
                {renderItem(item, virtualRow.index)}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
