"use client";

import { useMemo, useState } from "react";
import { RequireAuth } from "@/components/auth/RequireAuth";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

const CONFIG_TOOLS = [
  {
    value: "runtime",
    title: "Runtime config",
    description: "Inspect and update runtime overrides.",
    href: "/dev/config",
  },
  {
    value: "apps",
    title: "Apps config",
    description: "Manage app connector definitions.",
    href: "/dev/apps-config",
  },
  {
    value: "context-files",
    title: "Context config",
    description: "Edit persona/goal/world context YAML files.",
    href: "/dev/context-config",
  },
  {
    value: "context-window",
    title: "Context window",
    description: "Preview assembled context window for sessions.",
    href: "/dev/context-window",
  },
] as const;

export default function ConfigurationWorkbenchPage() {
  const [activeTool, setActiveTool] = useState<(typeof CONFIG_TOOLS)[number]["value"]>("runtime");

  const selected = useMemo(
    () => CONFIG_TOOLS.find((tool) => tool.value === activeTool) ?? CONFIG_TOOLS[0],
    [activeTool],
  );

  return (
    <RequireAuth>
      <div className="min-h-screen bg-slate-50 px-4 py-8 lg:px-8">
        <div className="mx-auto flex max-w-[1400px] flex-col gap-6">
          <header className="rounded-2xl bg-white/90 p-6 ring-1 ring-slate-200/60">
            <p className="text-[11px] font-semibold text-slate-400">Dev Tools · Configuration Workbench</p>
            <h1 className="mt-2 text-xl font-semibold text-slate-900 lg:text-2xl">Runtime and context configuration</h1>
            <p className="mt-2 text-sm text-slate-600">
              Unified workspace for runtime overrides, app connectors, and context configuration.
            </p>
          </header>

          <Card>
            <CardHeader className="space-y-3">
              <div className="flex items-center justify-between gap-2">
                <div>
                  <CardTitle className="text-base">Configuration tools</CardTitle>
                  <CardDescription>Switch tabs to use existing configuration editors in one place.</CardDescription>
                </div>
                <Badge variant="outline">{selected.title}</Badge>
              </div>
            </CardHeader>
            <CardContent>
              <Tabs value={activeTool} onValueChange={(value) => setActiveTool(value as typeof activeTool)}>
                <TabsList className="mb-4 flex h-auto flex-wrap gap-2 bg-transparent p-0">
                  {CONFIG_TOOLS.map((tool) => (
                    <TabsTrigger key={tool.value} value={tool.value} className="border border-slate-200 bg-white text-xs">
                      {tool.title}
                    </TabsTrigger>
                  ))}
                </TabsList>

                {CONFIG_TOOLS.map((tool) => (
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
