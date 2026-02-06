import type { HomeLang } from "../types";

export interface GLHomeCopy {
  title: string;
  tagline: string;
  cta: string;
  ctaHref: string;
  keywords: string[];
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
