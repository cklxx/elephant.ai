import type { HomeLang } from "../types";

export interface GLHomeCopy {
  title: string;
  tagline: string;
  cta: string;
  ctaHref: string;
  keywords: string[];
}

export interface GLSectionCopy {
  title: string;
  description: string;
  points: { title: string; description: string }[];
  scrollRange: { from: number; distance: number };
}

export const glCopy: Record<HomeLang, GLHomeCopy> = {
  en: {
    title: "elephant.ai",
    tagline: "Your AI teammate, always on.",
    cta: "Get Started →",
    ctaHref: "/conversation",
    keywords: [
      "Persistent Memory",
      "Autonomous Execution",
      "Context-Aware",
      "Lark-Native",
      "Proactive Agent",
    ],
  },
  zh: {
    title: "elephant.ai",
    tagline: "你的 AI 队友，永远在线。",
    cta: "开始使用 →",
    ctaHref: "/conversation",
    keywords: [
      "持续记忆",
      "自主执行",
      "上下文感知",
      "飞书原生",
      "主动代理",
    ],
  },
};

export const glSections: Record<HomeLang, GLSectionCopy[]> = {
  en: [
    {
      title: "Lives where you work",
      description: "No new app. No context switch. Talk to it in your existing Lark groups and DMs.",
      points: [
        { title: "Zero friction", description: "Works inside your existing groups and DMs — nothing to install." },
        { title: "Remembers context", description: "Picks up where you left off, across conversations and sessions." },
        { title: "Acts on your behalf", description: "Search, code, draft documents — from message to deliverable." },
      ],
      scrollRange: { from: 0.2, distance: 0.2 },
    },
    {
      title: "An agent that ships real work",
      description: "Beyond chat — built-in skills and a rich toolset for real output.",
      points: [
        { title: "Deep research", description: "Multi-step web search and synthesis, auto-generates reports." },
        { title: "Skill-driven", description: "Meeting notes, email drafts, slide decks — triggered by a message." },
        { title: "Extensible tools", description: "Code execution, file ops, browser automation, MCP plugins." },
      ],
      scrollRange: { from: 0.4, distance: 0.2 },
    },
    {
      title: "Autonomous, with guardrails",
      description: "Full autonomy when safe. Human approval when it matters.",
      points: [
        { title: "Persistent memory", description: "Remembers decisions and context across weeks and months." },
        { title: "Transparent execution", description: "Real-time progress, cost tracking, full audit trail." },
        { title: "Approval gates", description: "Risky actions require explicit human sign-off." },
      ],
      scrollRange: { from: 0.6, distance: 0.2 },
    },
  ],
  zh: [
    {
      title: "住在你的工作流里",
      description: "不用切应用、不用换标签页。在飞书群聊和私信里直接对话。",
      points: [
        { title: "零摩擦", description: "在已有的群聊和私信里直接使用——无需安装。" },
        { title: "记得上下文", description: "跨对话、跨会话延续记忆，不用重复说明。" },
        { title: "替你行动", description: "搜索、写代码、生成文档——从消息到交付物。" },
      ],
      scrollRange: { from: 0.2, distance: 0.2 },
    },
    {
      title: "不只聊天，是能交付的 Agent",
      description: "超越对话——内置技能和丰富工具，产出真实成果。",
      points: [
        { title: "深度研究", description: "多步骤搜索与信息综合，自动生成研究报告。" },
        { title: "技能驱动", description: "会议纪要、邮件撰写、PPT 生成——一句话触发。" },
        { title: "可扩展工具", description: "代码执行、文件操作、浏览器自动化、MCP 插件。" },
      ],
      scrollRange: { from: 0.4, distance: 0.2 },
    },
    {
      title: "自主运行，安全可控",
      description: "安全时全自动，关键时刻需要你的确认。",
      points: [
        { title: "持续记忆", description: "跨越数周数月，记住决策和上下文。" },
        { title: "执行透明", description: "进度实时可见、成本可查、全程审计。" },
        { title: "审批门控", description: "高风险操作需要明确的人工批准。" },
      ],
      scrollRange: { from: 0.6, distance: 0.2 },
    },
  ],
};
