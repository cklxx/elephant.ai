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
}

export const glCopy: Record<HomeLang, GLHomeCopy> = {
  en: {
    title: "elephant.ai",
    tagline: "Proactive AI that lives inside your workflow",
    cta: "Enter Console →",
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
    tagline: "住在工作流里的主动型 AI",
    cta: "进入控制台 →",
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
      title: "Lives in Lark",
      description: "Always online in your groups and DMs — responds like a team member.",
      points: [
        { title: "Zero switching cost", description: "No new app to open — talk in your existing groups and DMs." },
        { title: "Proactive context", description: "Auto-fetches recent chat history and cross-session memory." },
        { title: "Autonomous execution", description: "Search, code, generate documents — from a message to deliverable output." },
      ],
    },
    {
      title: "Not just chat — an agent that gets things done",
      description: "Built-in skills and a rich toolset to handle real work.",
      points: [
        { title: "Deep research", description: "Multi-step web search and synthesis, auto-generates research reports." },
        { title: "Skill-driven", description: "Meeting notes, email drafting, slide decks — triggered by natural language." },
        { title: "Rich toolset", description: "Code execution, file ops, browser automation, MCP extensions." },
      ],
    },
    {
      title: "AI inside your workflow, not outside it",
      description: "Don't let AI live in another app, another tab, another context switch.",
      points: [
        { title: "Persistent memory", description: "Remembers conversations, decisions, and context across sessions." },
        { title: "Fully observable", description: "Real-time progress, transparent cost and token tracking." },
        { title: "Approval gates", description: "Risky actions require explicit human approval." },
      ],
    },
  ],
  zh: [
    {
      title: "住在飞书里",
      description: "通过 WebSocket 常驻群聊和私信，像团队成员一样随时在线。",
      points: [
        { title: "零切换成本", description: "不需要打开新应用——在已有的群聊和私信里直接对话。" },
        { title: "主动理解上下文", description: "自动获取近期聊天记录、跨会话记忆，不用复述背景。" },
        { title: "自主执行工作", description: "搜索、写代码、生成文档——从一条消息到可交付产出。" },
      ],
    },
    {
      title: "不只是聊天——是能做事的 Agent",
      description: "内置技能和丰富工具，处理真实工作。",
      points: [
        { title: "深度研究", description: "多步骤网络搜索与信息综合，自动生成研究报告。" },
        { title: "技能驱动", description: "会议纪要、邮件撰写、PPT 生成——用自然语言触发。" },
        { title: "工具丰富", description: "代码执行、文件操作、浏览器自动化、MCP 扩展。" },
      ],
    },
    {
      title: "工作流里的 AI，而不是工作流外的",
      description: "别让 AI 在另一个应用、另一个标签页、另一次上下文切换里。",
      points: [
        { title: "持续记忆", description: "跨会话记住对话、决策和上下文，再也不用重复说明。" },
        { title: "全程可观测", description: "执行进度实时反馈、成本与 token 透明可查。" },
        { title: "审批门控", description: "高风险操作需要明确的人工审批。" },
      ],
    },
  ],
};
