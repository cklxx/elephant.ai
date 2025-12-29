import type { ChatTurn, StageCopy } from "./LiveChatShowcase";
import type { HomeLang } from "./types";

type DemoCopy = {
  title: string;
  description: string;
  userLabel: string;
  agentLabel: string;
  toolLabel: string;
  evidenceLabel: string;
};

export const liveChatCopy: Record<HomeLang, DemoCopy> = {
  zh: {
    title: "Live demo: 以对话框展示 Plan → Clearify → ReAct",
    description: "直接看对话框里如何声明任务、调用工具、记录证据。",
    userLabel: "用户",
    agentLabel: "Agent",
    toolLabel: "Tool",
    evidenceLabel: "证据流",
  },
  en: {
    title: "Live demo: Plan → Clearify → ReAct inside chat",
    description: "Watch the conversation declare tasks, call tools, and log evidence.",
    userLabel: "User",
    agentLabel: "Agent",
    toolLabel: "Tool",
    evidenceLabel: "Evidence",
  },
};

export const liveChatStages: Record<HomeLang, StageCopy[]> = {
  zh: [
    {
      key: "plan",
      label: "Plan",
      summary: "声明目标 + 约束",
      accent: "from-indigo-500 via-sky-500 to-emerald-500",
    },
    {
      key: "clearify",
      label: "Clearify",
      summary: "拆成可验收任务",
      accent: "from-amber-500 via-orange-500 to-rose-500",
    },
    {
      key: "react",
      label: "ReAct",
      summary: "交替推理与行动",
      accent: "from-emerald-500 via-teal-500 to-cyan-500",
    },
  ],
  en: [
    {
      key: "plan",
      label: "Plan",
      summary: "Declare goal + constraints",
      accent: "from-indigo-500 via-sky-500 to-emerald-500",
    },
    {
      key: "clearify",
      label: "Clearify",
      summary: "Break into reviewable tasks",
      accent: "from-amber-500 via-orange-500 to-rose-500",
    },
    {
      key: "react",
      label: "ReAct",
      summary: "Alternate reasoning/actions",
      accent: "from-emerald-500 via-teal-500 to-cyan-500",
    },
  ],
};

export const liveChatScript: Record<HomeLang, ChatTurn[]> = {
  zh: [
    {
      role: "user",
      content: "请在首页用对话框演示 Plan/Clearify/ReAct，少一点文字，多一些动态。",
      stage: "plan",
    },
    {
      role: "agent",
      content: "已对齐目标：用实时对话框展示流程，保持中英双语一致，首页聚焦演示。",
      stage: "plan",
    },
    {
      role: "agent",
      content: "任务声明：1) 自动播放三阶段 2) 支持手动暂停/重置 3) 工具调用要可见。",
      stage: "clearify",
    },
    {
      role: "tool",
      content: "tools.orchestrate(goal='live_demo', stages=['Plan','Clearify','ReAct']) → ok",
      stage: "clearify",
    },
    {
      role: "agent",
      content: "执行：正在播放阶段卡片，切换时更新行动日志与证据面板。",
      stage: "react",
    },
    {
      role: "tool",
      content: "tools.log(step='ReAct', evidence='action_log_synced') → recorded",
      stage: "react",
    },
    {
      role: "agent",
      content: "结果：演示完成，可复现，用户可随时暂停或回放。",
      stage: "react",
    },
  ],
  en: [
    {
      role: "user",
      content: "Show Plan/Clearify/ReAct on the homepage as a chat-style demo. Less copy, more motion.",
      stage: "plan",
    },
    {
      role: "agent",
      content: "Aligned goal: live chat box shows the flow; keep zh/en in sync; homepage centered on the demo.",
      stage: "plan",
    },
    {
      role: "agent",
      content: "Declaring tasks: 1) autoplay the three stages 2) allow pause/reset 3) surface tool calls.",
      stage: "clearify",
    },
    {
      role: "tool",
      content: "tools.orchestrate(goal='live_demo', stages=['Plan','Clearify','ReAct']) → ok",
      stage: "clearify",
    },
    {
      role: "agent",
      content: "Executing: playing stage cards; switching updates the action log and evidence lane.",
      stage: "react",
    },
    {
      role: "tool",
      content: "tools.log(step='ReAct', evidence='action_log_synced') → recorded",
      stage: "react",
    },
    {
      role: "agent",
      content: "Result: demo complete, replayable, user can pause or restart anytime.",
      stage: "react",
    },
  ],
};
