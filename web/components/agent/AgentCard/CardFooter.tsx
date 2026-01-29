import React from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";

interface CardFooterProps {
  expanded: boolean;
  onToggle: () => void;
  eventCount: number;
}

export function CardFooter({ expanded, onToggle, eventCount }: CardFooterProps) {
  if (eventCount === 0) {
    return null;
  }

  const label = expanded
    ? "Collapse"
    : eventCount === 1
      ? "Expand"
      : `${eventCount} events`;

  return (
    <div className="pl-[22px] py-0.5">
      <button
        type="button"
        onClick={onToggle}
        className="inline-flex items-center gap-1 text-[11px] text-muted-foreground/50 hover:text-muted-foreground transition-colors cursor-pointer"
      >
        <span>{label}</span>
        <ChevronDown
          className={cn(
            "h-3 w-3 transition-transform",
            expanded && "rotate-180",
          )}
        />
      </button>
    </div>
  );
}
