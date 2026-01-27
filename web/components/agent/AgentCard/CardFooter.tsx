import React from "react";
import { Button } from "@/components/ui/button";
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

  const buttonText = eventCount === 1
    ? (expanded ? "Collapse" : "Show full content")
    : (expanded ? "Show only latest" : `Show all ${eventCount} events`);

  return (
    <div className="flex items-center justify-between border-t border-border/30">
      <Button
        variant="ghost"
        size="sm"
        onClick={onToggle}
        className={cn(
          "h-7 text-xs text-muted-foreground hover:text-foreground transition-colors",
          "w-full justify-between",
        )}
      >
        <span>{buttonText}</span>
        <span className={cn("transition-transform", expanded && "rotate-180")}>
          ▼
        </span>
      </Button>
    </div>
  );
}
