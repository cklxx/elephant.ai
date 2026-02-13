"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Activity, BarChart2, Clock, PlayCircle, RefreshCw } from "lucide-react";


import { PageContainer, PageShell, SectionBlock, SectionHeader } from "@/components/layout/page-shell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/use-toast";
import { getEvaluation, listEvaluations, startEvaluation } from "@/lib/api";
import {
  AgentProfile,
  EvaluationDetailResponse,
  EvaluationJobSummary,
  EvaluationWorkerResultSummary,
  StartEvaluationRequest,
} from "@/lib/types";
import { formatDuration } from "@/lib/utils";

const formatDurationFromNs = (value?: number | null) => {
  if (!value) return "—";
  return formatDuration(value / 1_000_000);
};

const defaultRequest: StartEvaluationRequest = {
  dataset_path: "./evaluation/swe_bench/real_instances.json",
  instance_limit: 3,
  max_workers: 2,
  timeout_seconds: 300,
  enable_metrics: true,
  report_format: "markdown",
  agent_id: "default-agent",
};

export default function EvaluationPage() {
  const { toast } = useToast();
  const [evaluations, setEvaluations] = useState<EvaluationJobSummary[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detail, setDetail] = useState<EvaluationDetailResponse | null>(null);
  const [loadingList, setLoadingList] = useState<boolean>(true);
  const [loadingDetail, setLoadingDetail] = useState<boolean>(false);
  const [form, setForm] = useState<StartEvaluationRequest>(defaultRequest);
  const [starting, setStarting] = useState<boolean>(false);

  const sortedEvaluations = useMemo(() => {
    return [...evaluations].sort((a, b) => {
      const aTime = a.started_at ? new Date(a.started_at).getTime() : 0;
      const bTime = b.started_at ? new Date(b.started_at).getTime() : 0;
      return bTime - aTime;
    });
  }, [evaluations]);

  const refreshEvaluations = useCallback(async () => {
    setLoadingList(true);
    try {
      const data = await listEvaluations();
      setEvaluations(data.evaluations || []);
      if (data.evaluations?.length) {
        setSelectedId((prev) => prev ?? data.evaluations[0].id);
      }
    } catch (error) {
      console.error("Failed to load evaluations", error);
      toast({
        title: "无法加载评估列表",
        description: error instanceof Error ? error.message : "请求失败",
        variant: "destructive",
      });
    } finally {
      setLoadingList(false);
    }
  }, [toast]);

  const refreshDetail = useCallback(async (id: string) => {
    setLoadingDetail(true);
    try {
      const data = await getEvaluation(id);
      setDetail(data);
    } catch (error) {
      console.error("Failed to load evaluation detail", error);
      toast({
        title: "无法加载评估详情",
        description: error instanceof Error ? error.message : "请求失败",
        variant: "destructive",
      });
    } finally {
      setLoadingDetail(false);
    }
  }, [toast]);

  useEffect(() => {
    void refreshEvaluations();
    const timer = setInterval(() => {
      void refreshEvaluations();
    }, 8000);
    return () => clearInterval(timer);
  }, [refreshEvaluations]);

  useEffect(() => {
    if (!selectedId) return;
    void refreshDetail(selectedId);
  }, [selectedId, refreshDetail]);

  const handleStart = async () => {
    setStarting(true);
    try {
      const job = await startEvaluation(form);
      toast({ title: "已启动评估", description: `任务 ${job.id} 已开始` });
      setSelectedId(job.id);
      await refreshEvaluations();
      await refreshDetail(job.id);
    } catch (error) {
      console.error("Failed to start evaluation", error);
      toast({
        title: "无法启动评估",
        description: error instanceof Error ? error.message : "请求失败",
        variant: "destructive",
      });
    } finally {
      setStarting(false);
    }
  };

  const selectedJob = selectedId
    ? evaluations.find((item) => item.id === selectedId) ?? detail?.evaluation
    : null;

  return (
      <PageShell>
        <PageContainer>
          <SectionBlock>
            <SectionHeader
              overline="Agent evaluation"
              title="评估面板（Web 模式）"
              description="直接在浏览器发起评估任务，查看运行轨迹、得分、指标和改进建议。"
              titleElement="h1"
              actions={
                <Button variant="outline" onClick={refreshEvaluations} disabled={loadingList}>
                  <RefreshCw className="mr-2 h-4 w-4" /> 刷新列表
                </Button>
              }
            />
          </SectionBlock>

          <div className="grid gap-4 lg:grid-cols-3">
            <Card className="lg:col-span-2">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <PlayCircle className="h-5 w-5" />
                  发起一次评估
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="grid gap-3 md:grid-cols-2">
                  <div className="space-y-1">
                    <label
                      htmlFor="evaluation-dataset-path"
                      className="text-sm text-muted-foreground"
                    >
                      数据集路径
                    </label>
                    <Input
                      id="evaluation-dataset-path"
                      name="dataset_path"
                      value={form.dataset_path ?? ""}
                      onChange={(e) => setForm((prev) => ({ ...prev, dataset_path: e.target.value }))}
                      placeholder="./evaluation/swe_bench/real_instances.json"
                    />
                  </div>
                  <div className="space-y-1">
                    <label
                      htmlFor="evaluation-agent-id"
                      className="text-sm text-muted-foreground"
                    >
                      Agent ID
                    </label>
                    <Input
                      id="evaluation-agent-id"
                      name="agent_id"
                      value={form.agent_id ?? ""}
                      onChange={(e) => setForm((prev) => ({ ...prev, agent_id: e.target.value }))}
                      placeholder="default-agent"
                    />
                  </div>
                  <div className="space-y-1">
                    <label
                      htmlFor="evaluation-output-dir"
                      className="text-sm text-muted-foreground"
                    >
                      输出目录
                    </label>
                    <Input
                      id="evaluation-output-dir"
                      name="output_dir"
                      value={form.output_dir ?? ""}
                      onChange={(e) => setForm((prev) => ({ ...prev, output_dir: e.target.value }))}
                      placeholder="./evaluation_results"
                    />
                  </div>
                </div>

                <div className="grid gap-3 md:grid-cols-3">
                  <NumberInput
                    id="evaluation-instance-limit"
                    label="实例数量"
                    value={form.instance_limit ?? 0}
                    onChange={(value) => setForm((prev) => ({ ...prev, instance_limit: value }))}
                  />
                  <NumberInput
                    id="evaluation-max-workers"
                    label="并发 worker"
                    value={form.max_workers ?? 0}
                    onChange={(value) => setForm((prev) => ({ ...prev, max_workers: value }))}
                  />
                  <NumberInput
                    id="evaluation-timeout-seconds"
                    label="单任务超时（秒）"
                    value={form.timeout_seconds ?? 0}
                    onChange={(value) => setForm((prev) => ({ ...prev, timeout_seconds: value }))}
                  />
                </div>

                <div className="flex items-center gap-2">
                  <input
                    id="metrics-toggle"
                    type="checkbox"
                    className="h-4 w-4"
                    checked={form.enable_metrics ?? true}
                    onChange={(e) => setForm((prev) => ({ ...prev, enable_metrics: e.target.checked }))}
                  />
                  <label htmlFor="metrics-toggle" className="text-sm text-muted-foreground">
                    启用指标收集与报告生成
                  </label>
                </div>

                <div className="flex items-center gap-3">
                  <Button onClick={handleStart} disabled={starting}>
                    {starting ? "启动中..." : "开始评估"}
                  </Button>
                  <p className="text-sm text-muted-foreground">
                    默认使用 SWE-Bench 样例数据，可调整实例数量快速出报告。
                  </p>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between">
                <CardTitle className="flex items-center gap-2">
                  <Activity className="h-5 w-5" />
                  任务列表
                </CardTitle>
                <Badge variant="secondary">{sortedEvaluations.length} runs</Badge>
              </CardHeader>
              <CardContent className="space-y-2">
                {loadingList ? (
                  <div className="space-y-2">
                    <Skeleton className="h-10 w-full" />
                    <Skeleton className="h-10 w-full" />
                    <Skeleton className="h-10 w-full" />
                  </div>
                ) : sortedEvaluations.length === 0 ? (
                  <p className="text-sm text-muted-foreground">暂无评估任务，点击上方启动一个。</p>
                ) : (
                  sortedEvaluations.map((job) => (
                    <button
                      key={job.id}
                      onClick={() => setSelectedId(job.id)}
                      className={`w-full rounded-xl border px-3 py-2 text-left transition hover:border-primary ${
                        selectedId === job.id ? "border-primary bg-primary/5" : "border-border"
                      }`}
                    >
                      <div className="flex items-center justify-between text-sm">
                        <div className="flex items-center gap-2">
                          <Badge variant="outline">{job.id}</Badge>
                          {job.started_at ? (
                            <span className="text-muted-foreground">
                              {new Date(job.started_at).toLocaleString()}
                            </span>
                          ) : null}
                        </div>
                        <StatusPill status={job.status} />
                      </div>
                      {job.summary ? (
                        <div className="mt-1 text-xs text-muted-foreground">
                          总分 {(job.summary.overall_score * 100).toFixed(1)}% · 风险 {job.summary.risk_level}
                        </div>
                      ) : null}
                    </button>
                  ))
                )}
              </CardContent>
            </Card>
          </div>

          <Card className="mt-4">
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="flex items-center gap-2">
                <BarChart2 className="h-5 w-5" />
                运行详情
              </CardTitle>
              {selectedJob ? <StatusPill status={selectedJob.status} /> : null}
            </CardHeader>
            <CardContent>
              {loadingDetail ? (
                <div className="space-y-2">
                  <Skeleton className="h-6 w-1/3" />
                  <Skeleton className="h-10 w-full" />
                  <Skeleton className="h-32 w-full" />
                </div>
              ) : selectedJob ? (
                <div className="space-y-4">
                  <div className="grid gap-3 md:grid-cols-4">
                    <StatBlock
                      label="总体得分"
                      value={selectedJob.summary ? `${(selectedJob.summary.overall_score * 100).toFixed(1)}%` : "待生成"}
                      hint={selectedJob.summary?.performance_grade}
                    />
                  <StatBlock
                    label="成功率"
                    value={selectedJob.metrics ? `${(selectedJob.metrics.performance.success_rate * 100).toFixed(1)}%` : "—"}
                    hint={`P95 ${formatDurationFromNs(selectedJob.metrics?.performance.p95_time)}`}
                  />
                    <StatBlock
                      label="平均成本"
                      value={selectedJob.metrics ? `$${selectedJob.metrics.resources.avg_cost_per_task.toFixed(3)}` : "—"}
                      hint={`Tokens ${selectedJob.metrics?.resources.avg_tokens_used ?? 0}`}
                    />
                    <StatBlock
                      label="并发与超时"
                      value={`${selectedJob.max_workers ?? "?"} workers`}
                      hint={`超时 ${selectedJob.timeout_seconds ?? 0}s`}
                    />
                  </div>

                  {detail?.analysis ? (
                    <div className="grid gap-3 md:grid-cols-2">
                      <InsightList
                        title="优势"
                        items={detail.analysis.summary.key_strengths ?? []}
                        emptyLabel="暂无优势数据"
                      />
                      <InsightList
                        title="弱点"
                        items={detail.analysis.summary.key_weaknesses ?? []}
                        emptyLabel="暂无弱点数据"
                      />
                    </div>
                  ) : null}

                  {detail?.agent ? <AgentProfileCard agent={detail.agent} /> : null}

                  <ResultTable results={detail?.results ?? []} />
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">选择左侧的任务查看指标与评分。</p>
              )}
            </CardContent>
          </Card>
        </PageContainer>
      </PageShell>
  );
}

function NumberInput({
  id,
  label,
  value,
  onChange,
}: {
  id: string;
  label: string;
  value: number;
  onChange: (value: number) => void;
}) {
  return (
    <div className="space-y-1">
      <label htmlFor={id} className="text-sm text-muted-foreground">
        {label}
      </label>
      <Input
        id={id}
        name={id}
        type="number"
        value={value}
        min={0}
        onChange={(e) => onChange(Number(e.target.value))}
      />
    </div>
  );
}

function StatusPill({ status }: { status: string }) {
  const tone =
    status === "completed"
      ? "bg-emerald-50 text-emerald-700 border-emerald-200"
      : status === "running"
        ? "bg-blue-50 text-blue-700 border-blue-200"
        : status === "failed"
          ? "bg-red-50 text-red-700 border-red-200"
          : "bg-slate-50 text-slate-700 border-border";

  const labelMap: Record<string, string> = {
    completed: "已完成",
    running: "运行中",
    pending: "排队中",
    failed: "失败",
  };

  return (
    <span className={`rounded-full border px-3 py-1 text-xs font-semibold ${tone}`}>
      {labelMap[status] ?? status}
    </span>
  );
}

function StatBlock({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="rounded-xl border border-border bg-muted/40 p-3">
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className="text-lg font-semibold text-foreground">{value}</div>
      {hint ? <div className="text-xs text-muted-foreground">{hint}</div> : null}
    </div>
  );
}

function InsightList({
  title,
  items,
  emptyLabel,
}: {
  title: string;
  items: string[];
  emptyLabel: string;
}) {
  return (
    <div className="rounded-xl border border-border p-3">
      <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
        <Clock className="h-4 w-4" /> {title}
      </div>
      {items.length === 0 ? (
        <p className="mt-2 text-sm text-muted-foreground">{emptyLabel}</p>
      ) : (
        <ul className="mt-2 space-y-1 text-sm text-foreground">
          {items.map((item) => (
            <li key={item} className="rounded-lg bg-muted/60 px-2 py-1">
              {item}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function AgentProfileCard({ agent }: { agent: AgentProfile }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <Activity className="h-4 w-4" /> Agent 画像
          <Badge variant="outline">{agent.agent_id}</Badge>
        </CardTitle>
        <p className="text-sm text-muted-foreground">自动评分会更新画像，便于长期跟踪。</p>
      </CardHeader>
      <CardContent className="grid gap-3 md:grid-cols-4">
        <StatBlock
          label="评估次数"
          value={`${agent.evaluation_count ?? 1} 次`}
          hint={agent.last_evaluated ? `最近 ${new Date(agent.last_evaluated).toLocaleString()}` : undefined}
        />
        <StatBlock
          label="平均成功率"
          value={agent.avg_success_rate !== undefined ? `${(agent.avg_success_rate * 100).toFixed(1)}%` : "—"}
        />
        <StatBlock
          label="平均成本"
          value={agent.avg_cost_per_task !== undefined ? `$${agent.avg_cost_per_task.toFixed(3)}` : "—"}
        />
        <StatBlock
          label="自动评分均值"
          value={agent.avg_quality_score !== undefined ? `${(agent.avg_quality_score * 100).toFixed(1)}分` : "—"}
        />
      </CardContent>
    </Card>
  );
}

function ResultTable({ results }: { results: EvaluationWorkerResultSummary[] }) {
  if (!results || results.length === 0) {
    return <p className="text-sm text-muted-foreground">等待结果生成后会显示实例级别的轨迹摘要。</p>;
  }

  return (
    <div className="overflow-hidden rounded-xl border border-border">
      <div className="grid grid-cols-7 bg-muted px-3 py-2 text-xs font-semibold text-muted-foreground">
        <span>Instance</span>
        <span>Status</span>
        <span>Duration</span>
        <span>Tokens</span>
        <span>Cost</span>
        <span>Auto Score</span>
        <span>Error</span>
      </div>
      <div className="divide-y divide-border">
        {results.map((result) => (
          <div key={result.task_id} className="grid grid-cols-7 items-center px-3 py-2 text-sm">
            <span className="truncate" title={result.instance_id}>
              {result.instance_id}
            </span>
            <StatusPill status={result.status} />
            <span>
              {result.duration_seconds
                ? formatDuration(result.duration_seconds * 1000)
                : "—"}
            </span>
            <span>{result.tokens_used ?? "-"}</span>
            <span>{result.cost ? `$${result.cost.toFixed(3)}` : "-"}</span>
            <span className="flex items-center gap-1">
              {result.auto_score !== undefined
                ? `${result.auto_score.toFixed(1)}`
                : "-"}
              {result.grade ? (
                <Badge variant="outline" className="text-[11px]">
                  {result.grade}
                </Badge>
              ) : null}
            </span>
            <span className="truncate" title={result.error}>
              {result.error ? result.error : ""}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
