"use client";

import { useMemo, useState } from "react";
import { ChevronDown } from "lucide-react";

import { skillsCatalog } from "@/lib/skillsCatalog";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { LazyMarkdownRenderer } from "@/components/agent/LazyMarkdownRenderer";

export function SkillsPanel() {
  const skills = useMemo(() => skillsCatalog.skills ?? [], []);
  const [expandedSkill, setExpandedSkill] = useState<string | null>(null);

  if (!skills || skills.length === 0) {
    return null;
  }

  return (
    <Card className="rounded-2xl border border-border/60 bg-card">
      <CardHeader className="flex flex-row items-start justify-between gap-3 pb-4">
        <div className="space-y-1">
          <p className="text-sm font-semibold text-foreground">Skills</p>
          <p className="text-xs text-muted-foreground">
            Reusable playbooks shipped with this repository.
          </p>
        </div>
        <Badge variant="outline" className="rounded-full text-[11px]">
          {skills.length}
        </Badge>
      </CardHeader>
      <CardContent className="pt-0">
        <ScrollArea className="h-[70vh]">
          <div className="flex flex-col gap-3 pr-1">
            {skills.map((skill) => {
              const isExpanded = expandedSkill === skill.name;
              const title = skill.title?.trim() || skill.name;
              const description = skill.description?.trim();

              return (
                <div
                  key={skill.name}
                  className="rounded-2xl border border-border/70 bg-background p-3"
                >
                  <button
                    type="button"
                    onClick={() =>
                      setExpandedSkill((prev) =>
                        prev === skill.name ? null : skill.name,
                      )
                    }
                    className="flex w-full items-start justify-between gap-3 text-left"
                  >
                    <div className="min-w-0 space-y-1">
                      <p className="truncate text-sm font-semibold text-foreground">
                        {title}
                      </p>
                      {description && (
                        <p className="text-xs text-muted-foreground">
                          {description}
                        </p>
                      )}
                    </div>

                    <div className="flex shrink-0 items-center gap-2">
                      <Badge
                        variant="outline"
                        className="text-[11px] font-mono"
                      >
                        {skill.name}
                      </Badge>
                      <ChevronDown
                        className={cn(
                          "h-4 w-4 text-muted-foreground transition-transform",
                          isExpanded ? "rotate-180" : "rotate-0",
                        )}
                        aria-hidden="true"
                      />
                    </div>
                  </button>

                  {isExpanded && (
                    <div className="mt-3 border-t border-border/60 pt-3">
                      <LazyMarkdownRenderer
                        content={skill.markdown}
                        containerClassName="markdown-body text-sm"
                        className="prose prose-sm max-w-none text-foreground"
                      />
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </ScrollArea>
      </CardContent>
    </Card>
  );
}
