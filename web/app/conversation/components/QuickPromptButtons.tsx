import type { LucideIcon } from "lucide-react";

import { Button } from "@/components/ui/button";

export interface QuickPromptItem {
  id: string;
  label: string;
  icon: LucideIcon;
  prompt: string;
}

interface QuickPromptButtonsProps {
  items: QuickPromptItem[];
  onSelect: (prompt: string) => void;
}

export function QuickPromptButtons({ items, onSelect }: QuickPromptButtonsProps) {
  return (
    <div className="mt-3 flex flex-wrap justify-center gap-2">
      {items.map((item) => (
        <Button
          key={item.id}
          type="button"
          variant="outline"
          size="sm"
          className="h-9 rounded-full border-border/40 bg-secondary/40 text-xs font-semibold shadow-none hover:bg-secondary/60"
          onClick={() => onSelect(item.prompt)}
        >
          <item.icon className="h-4 w-4" aria-hidden />
          {item.label}
        </Button>
      ))}
    </div>
  );
}
