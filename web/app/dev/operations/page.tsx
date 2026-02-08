"use client";

import { useMemo, useState } from "react";
import { RequireAuth } from "@/components/auth/RequireAuth";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

const OP_TOOLS = [
  {
    value: "evaluation",
    title: "Evaluation",
    description: "Run and inspect evaluation jobs.",
    href: "/evaluation",
  },
  {
    value: "sessions",
    title: "Sessions",
    description: "Browse session archive and open details.",
    href: "/sessions",
  },
  {
    value: "plan-preview",
    title: "Plan preview",
    description: "Preview plan timeline UI rendering.",
    href: "/dev/plan-preview",
  },
] as const;

export default function OperationsWorkbenchPage() {
  const [activeTool, setActiveTool] = useState<(typeof OP_TOOLS)[number]["value"]>("evaluation");

  const selected = useMemo(
    () => OP_TOOLS.find((tool) => tool.value === activeTool) ?? OP_TOOLS[0],
    [activeTool],
  );

  return (
    <RequireAuth>
      <div className="min-h-screen bg-slate-50 px-4 py-8 lg:px-8">
        <div className="mx-auto flex max-w-[1400px] flex-col gap-6">
          <header className="rounded-2xl bg-white/90 p-6 ring-1 ring-slate-200/60">
            <p className="text-[11px] font-semibold text-slate-400">Dev Tools · Operations Workbench</p>
            <h1 className="mt-2 text-xl font-semibold text-slate-900 lg:text-2xl">Evaluation and runtime operations</h1>
            <p className="mt-2 text-sm text-slate-600">
              Operate evaluation runs, sessions, and plan rendering tools from one workspace.
            </p>
          </header>

          <Card>
            <CardHeader className="space-y-3">
              <div className="flex items-center justify-between gap-2">
                <div>
                  <CardTitle className="text-base">Operations tools</CardTitle>
                  <CardDescription>Switch tabs to open each operational surface inline.</CardDescription>
                </div>
                <Badge variant="outline">{selected.title}</Badge>
              </div>
            </CardHeader>
            <CardContent>
              <Tabs value={activeTool} onValueChange={(value) => setActiveTool(value as typeof activeTool)}>
                <TabsList className="mb-4 flex h-auto flex-wrap gap-2 bg-transparent p-0">
                  {OP_TOOLS.map((tool) => (
                    <TabsTrigger key={tool.value} value={tool.value} className="border border-slate-200 bg-white text-xs">
                      {tool.title}
                    </TabsTrigger>
                  ))}
                </TabsList>

                {OP_TOOLS.map((tool) => (
                  <TabsContent key={tool.value} value={tool.value} className="space-y-3">
                    <p className="text-xs text-slate-500">{tool.description}</p>
                    <div className="overflow-hidden rounded-xl border border-slate-200 bg-white">
                      <iframe
                        src={tool.href}
                        title={tool.title}
                        className="h-[72vh] w-full"
                        loading="lazy"
                      />
                    </div>
                  </TabsContent>
                ))}
              </Tabs>
            </CardContent>
          </Card>
        </div>
      </div>
    </RequireAuth>
  );
}
