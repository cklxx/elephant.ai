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
    title: "Live demo: 对话框演示 Plan → Clearify → ReAct",
    description: "少字多动，看任务声明、工具调用与证据。",
    userLabel: "用户",
    agentLabel: "Agent",
    toolLabel: "Tool",
    evidenceLabel: "证据流",
  },
  en: {
    title: "Live demo: Plan → Clearify → ReAct in chat",
    description: "Less copy, more motion—see tasks, tools, and evidence.",
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
      summary: "目标 + 约束",
      accent: "from-indigo-500 via-sky-500 to-emerald-500",
    },
    {
      key: "clearify",
      label: "Clearify",
      summary: "拆成小任务",
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
      summary: "Goal + guardrails",
      accent: "from-indigo-500 via-sky-500 to-emerald-500",
    },
    {
      key: "clearify",
      label: "Clearify",
      summary: "Slice tasks",
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
      content: "首页用对话框演示 Plan/Clearify/ReAct，要少字多动。",
      stage: "plan",
    },
    {
      role: "agent",
      content: "目标：实时对话，双语同步，首页聚焦演示。",
      stage: "plan",
    },
    {
      role: "agent",
      content: "任务：1) 自动播放三阶段 2) 可暂停/重置 3) 工具调用可见。",
      stage: "clearify",
    },
    {
      role: "tool",
      content: "tools.orchestrate(goal='live_demo', stages=['Plan','Clearify','ReAct']) → ok",
      stage: "clearify",
    },
    {
      role: "agent",
      content: "执行：阶段卡片滚动，行动日志和证据同步更新。",
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
      content: "Homepage chat demo for Plan/Clearify/ReAct—short copy, more motion.",
      stage: "plan",
    },
    {
      role: "agent",
      content: "Goal: live chat shows the flow; zh/en stay aligned; homepage stays focused.",
      stage: "plan",
    },
    {
      role: "agent",
      content: "Tasks: 1) autoplay stages 2) pause/reset controls 3) visible tool calls.",
      stage: "clearify",
    },
    {
      role: "tool",
      content: "tools.orchestrate(goal='live_demo', stages=['Plan','Clearify','ReAct']) → ok",
      stage: "clearify",
    },
    {
      role: "agent",
      content: "Executing: cycling stage cards; action log and evidence stay in sync.",
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
