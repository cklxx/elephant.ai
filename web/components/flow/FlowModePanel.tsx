"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowRight, RefreshCcw, Search, Wand2 } from "lucide-react";
import type Quill from "quill";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  analyzeSentences,
  buildFlowDraft,
  buildFlowOutline,
  buildSearchCues,
  computeDraftStats,
  detectBlockers,
  extractKeywords,
  splitSentences,
  tightenSentence,
} from "./flow-utils";

type FlowIntent =
  | { kind: "list"; hint: string }
  | { kind: "research"; hint: string }
  | { kind: "rewrite"; hint: string }
  | null;

function htmlToPlainText(html: string): string {
  const container = document.createElement("div");
  container.innerHTML = html;
  return container.innerText.replace(/\u00A0/g, " ").trim();
}

function detectIntent(text: string, html: string, recentInput?: string): FlowIntent {
  const trimmed = text.trim();
  if (!trimmed) return null;

  if (/<(ol|ul)[^>]*>/i.test(html) || /^[-*•]/m.test(trimmed) || recentInput === "list") {
    return { kind: "list", hint: "检测到清单编辑，已保持段落行距和分点解析" };
  }

  if (/[?？]/.test(trimmed) || /^(目标|目的|背景)/.test(trimmed) || recentInput === "research") {
    return { kind: "research", hint: "检测到提问语气，已生成搜索提示与意图摘要" };
  }

  if (trimmed.length > 360 || recentInput === "rewrite") {
    return { kind: "rewrite", hint: "检测到长句，建议一键紧凑或心流重排" };
  }

  return null;
}

export function FlowModePanel() {
  const initialText =
    "写作目的：展示产品的“心流写作模式”。\n语气：克制、务实、偏产品说明。\n\n初稿：\n我们希望提供一个纯净的写作界面，减少干扰，让用户专注于句子本身。通过拆分句子、生成搜索提示和自动润色，帮助用户让文章更流畅。把写作过程从“堆字”变为“连句”，形成一气呵成的心流。";
  const [draftHtml, setDraftHtml] = useState<string>("");
  const [intent, setIntent] = useState<FlowIntent>(null);
  const quillRef = useRef<Quill | null>(null);
  const editorContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const container = editorContainerRef.current;
    if (!container) return;

    let quillInstance: Quill | null = null;
    void import("quill").then(({ default: QuillImport }) => {
      quillInstance = new QuillImport(container, {
        theme: "snow",
        placeholder: "写作目标、受众、稿件正文……",
        modules: {
          toolbar: [
            [{ header: [2, 3, false] }],
            ["bold", "italic", "underline"],
            [{ list: "ordered" }, { list: "bullet" }],
            ["blockquote", "link"],
            ["clean"],
          ],
        },
      });

      quillRef.current = quillInstance;
      quillInstance.root.innerHTML = initialText
        .split(/\n{2,}/)
        .map((paragraph) => `<p>${paragraph.trim()}</p>`)
        .join("");
      setDraftHtml(quillInstance.root.innerHTML);

      quillInstance.on("text-change", (_delta, _old, source) => {
        if (source !== "user") return;
        const html = quillInstance?.root.innerHTML ?? "";
        setDraftHtml(html);
        setIntent(detectIntent(quillInstance?.getText() ?? "", html));
      });
    });

    return () => {
      quillInstance?.off("text-change");
      quillRef.current = null;
    };
  }, []);

  useEffect(() => {
    const quill = quillRef.current;
    if (!quill) return;
    if (quill.root.innerHTML === draftHtml) return;
    const selection = quill.getSelection();
    quill.clipboard.dangerouslyPasteHTML(draftHtml);
    if (selection) {
      quill.setSelection(selection);
    }
  }, [draftHtml]);

  const draftText = useMemo(() => htmlToPlainText(draftHtml), [draftHtml]);
  const sentences = useMemo(() => splitSentences(draftText), [draftText]);
  const insights = useMemo(() => analyzeSentences(sentences), [sentences]);
  const keywords = useMemo(() => extractKeywords(draftText, 10), [draftText]);
  const flowDraft = useMemo(() => buildFlowDraft(sentences), [sentences]);
  const outline = useMemo(() => buildFlowOutline(sentences), [sentences]);
  const searchCues = useMemo(() => buildSearchCues(sentences), [sentences]);
  const stats = useMemo(
    () => computeDraftStats(draftText, sentences, keywords),
    [draftText, sentences, keywords],
  );
  const blockers = useMemo(
    () => detectBlockers(draftText, sentences, keywords, stats.paragraphs),
    [draftText, sentences, keywords, stats.paragraphs],
  );

  const applyTightenAll = () => {
    const tightened = sentences.map((sentence) => tightenSentence(sentence)).join("\n");
    quillRef.current?.clipboard.dangerouslyPasteHTML(
      tightened
        .split(/\n{2,}/)
        .map((paragraph) => `<p>${paragraph.trim()}</p>`)
        .join(""),
    );
    const nextHtml = quillRef.current?.root.innerHTML ?? "";
    setDraftHtml(nextHtml);
    setIntent(detectIntent(htmlToPlainText(nextHtml), nextHtml, "rewrite"));
  };

  const applyFlowDraft = () => {
    if (!flowDraft) return;
    quillRef.current?.clipboard.dangerouslyPasteHTML(
      flowDraft
        .split(/\n{2,}/)
        .map((paragraph) => `<p>${paragraph.trim()}</p>`)
        .join(""),
    );
    const nextHtml = quillRef.current?.root.innerHTML ?? "";
    setDraftHtml(nextHtml);
    setIntent(detectIntent(htmlToPlainText(nextHtml), nextHtml, "rewrite"));
  };

  const applyToolbarFormat = (format: "bold" | "italic" | "list", value?: unknown) => {
    if (!quillRef.current) return;
    quillRef.current.focus();
    quillRef.current.format(format, value ?? true);
    const html = quillRef.current.root.innerHTML;
    setDraftHtml(html);
    setIntent(detectIntent(quillRef.current.getText(), html, format === "list" ? "list" : undefined));
  };

  const intentBadge =
    intent && (
      <div className="inline-flex items-center gap-2 rounded-full border border-emerald-500/30 bg-emerald-500/10 px-3 py-1 text-[11px] font-semibold text-emerald-600">
        <span>
          {intent.kind === "list"
            ? "检测到清单"
            : intent.kind === "research"
              ? "检测到提问/研究意图"
              : "检测到长句"}
        </span>
        <span className="text-foreground/70">{intent.hint}</span>
      </div>
    );

  return (
    <div className="flex flex-col gap-4 lg:grid lg:grid-cols-[1.05fr,0.95fr] lg:items-start">
      <Card className="bg-card/70 backdrop-blur">
        <CardHeader className="flex flex-col gap-3">
          <div className="flex items-center gap-2">
            <div className="inline-flex items-center gap-2 rounded-full bg-emerald-500/10 px-3 py-1 text-xs font-semibold text-emerald-500">
              <Wand2 className="h-3.5 w-3.5" aria-hidden />
              心流写作 · Quill
            </div>
            <span className="text-xs text-muted-foreground">
              纯写作辅助 · 所见即所得 · 句子解析
            </span>
          </div>
          <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <Badge variant="secondary" className="border border-border/50 bg-background/60">
              字符：{stats.characters}
            </Badge>
            <Badge variant="secondary" className="border border-border/50 bg-background/60">
              句子：{stats.sentences}
            </Badge>
            <Badge variant="secondary" className="border border-border/50 bg-background/60">
              段落：{stats.paragraphs}
            </Badge>
            <Badge variant="secondary" className="border border-border/50 bg-background/60">
              关键词：{stats.keywordCount}
            </Badge>
            <Badge variant="secondary" className="border border-border/50 bg-background/60">
              预计阅读：{stats.readingMinutes} 分钟
            </Badge>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="rounded-full"
              onClick={() => applyToolbarFormat("bold")}
            >
              加粗
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="rounded-full"
              onClick={() => applyToolbarFormat("italic")}
            >
              斜体
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="rounded-full"
              onClick={() => applyToolbarFormat("list", "bullet")}
            >
              列表
            </Button>
            {intentBadge}
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-2xl border border-border/70 bg-background/70 p-1">
            <div ref={editorContainerRef} className="min-h-[260px]" />
          </div>
          {blockers.length ? (
            <div className="space-y-3 rounded-2xl border border-amber-300/50 bg-amber-50/70 p-3 text-sm text-amber-900">
              <div className="text-xs font-semibold uppercase tracking-wide text-amber-800">
                卡点扫描
              </div>
              <div className="space-y-2">
                {blockers.map((blocker) => (
                  <div
                    key={blocker.id}
                    className="rounded-xl border border-amber-200 bg-white/70 p-3 shadow-[0_10px_30px_-24px_rgba(0,0,0,0.25)]"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <Badge variant="secondary" className="border-amber-200 bg-amber-100 text-amber-900">
                        {blocker.title}
                      </Badge>
                      {blocker.fixHint && (
                        <span className="text-[11px] text-amber-700">{blocker.fixHint}</span>
                      )}
                    </div>
                    <p className="mt-1 text-xs text-amber-800">{blocker.detail}</p>
                    {blocker.id === "structure" && flowDraft ? (
                      <Button
                        type="button"
                        size="sm"
                        className="mt-2 rounded-full"
                        onClick={applyFlowDraft}
                      >
                        一键心流重排
                      </Button>
                    ) : null}
                    {blocker.id === "expression" ? (
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        className="mt-2 rounded-full"
                        onClick={applyTightenAll}
                      >
                        压缩长句
                      </Button>
                    ) : null}
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <div className="rounded-2xl border border-emerald-200 bg-emerald-50/80 p-3 text-sm text-emerald-900">
              思路通畅，继续写下去吧。
            </div>
          )}
          <div className="flex flex-wrap items-center gap-3">
            <Button
              type="button"
              variant="default"
              size="sm"
              className="rounded-full"
              onClick={applyFlowDraft}
              disabled={!flowDraft}
            >
              <ArrowRight className="mr-2 h-4 w-4" aria-hidden />
              应用心流重排
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="rounded-full"
              onClick={applyTightenAll}
              disabled={!sentences.length}
            >
              <RefreshCcw className="mr-2 h-4 w-4" aria-hidden />
              一键紧凑
            </Button>
            <div className="inline-flex flex-wrap gap-2 text-xs text-muted-foreground">
              {keywords.map((keyword) => (
                <Badge
                  key={keyword}
                  variant="outline"
                  className="border-dashed bg-background/50"
                >
                  {keyword}
                </Badge>
              ))}
            </div>
          </div>
          {flowDraft ? (
            <div className="rounded-2xl border border-border/60 bg-muted/30 p-3 text-sm leading-6 text-foreground">
              <div className="mb-2 inline-flex items-center gap-2 rounded-full bg-background/70 px-3 py-1 text-[11px] font-semibold text-muted-foreground">
                <Wand2 className="h-3.5 w-3.5" aria-hidden />
                心流草稿
              </div>
              <p className="whitespace-pre-line text-muted-foreground">{flowDraft}</p>
            </div>
          ) : null}
        </CardContent>
      </Card>

      <div className="space-y-4">
        <Card className="bg-card/70 backdrop-blur">
          <CardHeader className="space-y-2">
            <CardTitle className="flex items-center gap-2 text-sm">
              <Search className="h-4 w-4" aria-hidden />
              搜索提示 · 表达查找
            </CardTitle>
            <p className="text-xs text-muted-foreground">
              自动抓取关键词，生成可直接搜的提示，便于扩写/找案例/换表达。
            </p>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-2">
            {searchCues.map((cue) => (
              <Badge
                key={cue}
                variant="secondary"
                className="rounded-full border border-border/60 bg-background/70 text-xs font-semibold text-foreground"
              >
                {cue}
              </Badge>
            ))}
            {!searchCues.length && (
              <span className="text-xs text-muted-foreground">输入草稿后自动生成。</span>
            )}
          </CardContent>
        </Card>

        <Card className="bg-card/70 backdrop-blur">
          <CardHeader className="space-y-2">
            <CardTitle className="text-sm font-semibold">句子解析 · 紧凑重写</CardTitle>
            <p className="text-xs text-muted-foreground">
              拆句、标注节奏、建议更紧凑的改写，方便你挑选。
            </p>
          </CardHeader>
          <CardContent className="space-y-3">
            {insights.map((insight) => (
              <div
                key={insight.original}
                className="rounded-2xl border border-border/60 bg-background/60 p-3"
              >
                <div className="flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
                  <Badge variant="outline" className="border-dashed">
                    {insight.rhythm === "short"
                      ? "短句"
                      : insight.rhythm === "balanced"
                        ? "均衡"
                        : "长句"}
                  </Badge>
                  {insight.keywords.map((keyword) => (
                    <Badge key={keyword} variant="secondary" className="border-border/60">
                      {keyword}
                    </Badge>
                  ))}
                  <span>长度：{insight.length}</span>
                </div>
                <p className="mt-2 text-sm font-medium text-foreground">{insight.original}</p>
                <p className="mt-1 text-sm text-emerald-500">{insight.tightened}</p>
              </div>
            ))}
            {!insights.length && (
              <p className="text-xs text-muted-foreground">输入一段文本即可生成句子解析。</p>
            )}
          </CardContent>
        </Card>

        <Card className="bg-card/70 backdrop-blur">
          <CardHeader className="space-y-2">
            <CardTitle className="text-sm font-semibold">行文脉络</CardTitle>
            <p className="text-xs text-muted-foreground">
              自动提取开场、展开、收束，保持文章心流。
            </p>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="grid gap-2 lg:grid-cols-3">
              {outline.map((item) => (
                <div
                  key={item}
                  className="rounded-xl border border-border/60 bg-background/60 p-3 text-sm text-foreground"
                >
                  {item}
                </div>
              ))}
              {!outline.length && (
                <div className="rounded-xl border border-dashed border-border/60 bg-muted/20 p-3 text-xs text-muted-foreground">
                  输入两句以上文本后自动生成开场-展开-收束。
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
