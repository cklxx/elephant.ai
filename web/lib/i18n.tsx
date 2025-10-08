'use client';

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { isBrowser } from '@/lib/utils';

export type Language = 'en' | 'zh';

const translations = {
  en: {
    'app.loading': 'Loading console…',
    'language.label': 'Language',
    'language.option.en': 'English',
    'language.option.zh': 'Chinese (Simplified)',
    'language.option.en.short': 'EN',
    'language.option.zh.short': '中',
    'timeline.label': 'Execution timeline',
    'timeline.waiting': 'Waiting for execution to begin',
    'timeline.status.inProgress': 'In progress: {title}',
    'timeline.status.attention': 'Needs attention: {title}',
    'timeline.status.recent': 'Recently completed: {title}',
    'timeline.progress': 'Completed {completed}/{total}',
    'console.title': 'ALEX Console',
    'console.heading': 'Operator Dashboard',
    'console.subtitle.active': 'Active session {id}',
    'console.subtitle.default':
      'Describe your work goal in English or Chinese to start a new research session.',
    'console.connection.title': 'Connection',
    'console.connection.subtitle': 'Live status',
    'console.connection.mock': 'Mock stream enabled',
    'console.connection.newConversation': 'Start new conversation',
    'console.settings.title': 'Workspace settings',
    'console.settings.subtitle': 'Manage language, sessions, and connection health.',
    'console.settings.sessionLabel': 'Current session',
    'console.settings.sessionEmpty': 'None',
    'console.history.title': 'Session history',
    'console.history.subtitle': 'Automatically saves the latest 10 sessions',
    'console.history.empty': 'No sessions yet.',
    'console.history.itemPrefix': 'Session {id}',
    'console.quickstart.title': 'Quick start',
    'console.quickstart.items.code': '• Code generation, debugging, and testing',
    'console.quickstart.items.docs': '• Documentation writing and research summaries',
    'console.quickstart.items.architecture': '• Architecture analysis and technical comparisons',
    'console.thread.title': 'Live Thread',
    'console.thread.sessionPrefix': 'Session {id}',
    'console.thread.newConversation': 'New research conversation',
    'console.thread.autosave': 'Autosave',
    'console.thread.subtitle.active':
      'Continue your research or ask for new assistance. ALEX preserves context and reasoning.',
    'console.thread.subtitle.idle':
      "Describe your goal and we'll create a plan, then execute it with tools.",
    'console.timeline.mobileLabel': 'Timeline snapshot',
    'console.timeline.dialogTitle': 'Execution timeline',
    'console.timeline.dialogDescription':
      'Review research progress on mobile. Tap any step to jump to the matching event.',
    'console.empty.badge': 'Waiting for your task',
    'console.empty.title': 'Ready to take on your work',
    'console.empty.description':
      'After you submit a task, session history appears on the left and plans, tool calls, and outputs render on the right.',
    'console.input.placeholder.active': 'Continue the conversation with a new request…',
    'console.input.placeholder.idle': 'Describe the task or question you want to work on…',
    'console.input.hotkeyHint': 'Press Enter to send · Shift+Enter for newline',
    'console.toast.taskFailed': 'Task execution failed',
    'connection.connected': 'Connected',
    'connection.reconnecting': 'Reconnecting… (Attempt {attempt})',
    'connection.disconnected': 'Disconnected',
    'connection.reconnect': 'Reconnect',
    'timeline.card.title': 'Execution Timeline',
    'timeline.card.subtitle': 'Track progress through each research step',
    'timeline.card.toolsUsed': 'Tools used:',
    'timeline.card.tokensUsed': 'Tokens used:',
    'timeline.card.error': 'Error:',
    'timeline.card.expand': 'Expand details',
    'timeline.card.collapse': 'Collapse details',
    'timeline.card.badge.pending': 'Pending',
    'timeline.card.badge.active': 'In progress',
    'timeline.card.badge.complete': 'Complete',
    'timeline.card.badge.error': 'Failed',
    'plan.title': 'Research Plan',
    'plan.caption.default': 'Review and approve to start execution',
    'plan.caption.readonly': 'Approved plan',
    'plan.collapse': 'Collapse plan',
    'plan.expand': 'Expand plan',
    'plan.goal.label': 'Goal:',
    'plan.steps.label': 'Planned steps ({count}):',
    'plan.iterations': 'Iterations:',
    'plan.tools': 'Tools:',
    'plan.tools.more': '+{count} more',
    'plan.actions.saveChanges': 'Save changes',
    'plan.actions.cancel': 'Cancel',
    'plan.actions.approve': 'Approve & start',
    'plan.actions.modify': 'Modify plan',
    'plan.actions.reject': 'Reject plan',
    'plan.reject.reasonLabel': 'Rejection reason',
    'plan.reject.placeholder': 'Why is this plan not ready to execute?',
    'plan.reject.confirm': 'Confirm rejection',
    'plan.reject.cancel': 'Cancel',
    'plan.edit.goal': 'Edit goal',
    'plan.edit.stepLabel': 'Edit step {index}',
    'plan.move.up': 'Move step {index} up',
    'plan.move.down': 'Move step {index} down',
    'task.submit.title.running': 'Running…',
    'task.submit.title.default': 'Submit (Enter)',
    'task.submit.running': 'Running',
    'task.submit.label': 'Send',
    'task.input.ariaLabel': 'Task input',
    'tool.status.failed': 'Failed',
    'tool.status.completed': 'Completed',
    'tool.toggle.expand': 'Expand output',
    'tool.toggle.collapse': 'Collapse output',
    'tool.toggle.length': ' · {count} characters',
    'tool.section.parameters': 'Parameters',
    'tool.section.error': 'Error',
    'tool.section.output': 'Output',
    'sessions.archiveLabel': 'Session archive',
    'sessions.title': 'Session management',
    'sessions.description': 'Review, revisit, and reopen ALEX automated workflows.',
    'sessions.newConversation': 'Start new conversation',
  },
  zh: {
    'app.loading': '加载控制台…',
    'language.label': '语言',
    'language.option.en': '英语',
    'language.option.zh': '简体中文',
    'language.option.en.short': 'EN',
    'language.option.zh.short': '中',
    'timeline.label': '执行时间线',
    'timeline.waiting': '等待执行开始',
    'timeline.status.inProgress': '进行中：{title}',
    'timeline.status.attention': '需要关注：{title}',
    'timeline.status.recent': '最近完成：{title}',
    'timeline.progress': '已完成 {completed}/{total}',
    'console.title': 'ALEX 控制台',
    'console.heading': '操作面板',
    'console.subtitle.active': '进行中的会话 {id}',
    'console.subtitle.default': '用中文或英文描述你的工作目标，开始新的研究会话。',
    'console.connection.title': '连接状态',
    'console.connection.subtitle': '实时状态',
    'console.connection.mock': '模拟流已启用',
    'console.connection.newConversation': '新建对话',
    'console.settings.title': '工作区设置',
    'console.settings.subtitle': '管理语言、会话与连接状态。',
    'console.settings.sessionLabel': '当前会话',
    'console.settings.sessionEmpty': '无',
    'console.history.title': '历史会话',
    'console.history.subtitle': '自动保存最近 10 个',
    'console.history.empty': '目前还没有历史会话。',
    'console.history.itemPrefix': '会话 {id}',
    'console.quickstart.title': '快速指引',
    'console.quickstart.items.code': '• 代码生成、调试与测试',
    'console.quickstart.items.docs': '• 文档撰写、研究总结',
    'console.quickstart.items.architecture': '• 架构分析与技术对比',
    'console.thread.title': '实时对话',
    'console.thread.sessionPrefix': '会话 {id}',
    'console.thread.newConversation': '新的研究对话',
    'console.thread.autosave': '自动保存',
    'console.thread.subtitle.active': '继续你的研究或提出新的请求。ALEX 将保持上下文并延续推理。',
    'console.thread.subtitle.idle': '描述你的目标，我们会生成执行计划并通过工具完成任务。',
    'console.timeline.mobileLabel': '时间线概览',
    'console.timeline.dialogTitle': '执行时间线',
    'console.timeline.dialogDescription': '在移动端查看研究步骤进度，点击任意步骤即可定位到对应的事件记录。',
    'console.empty.badge': '等待新的任务指令',
    'console.empty.title': '准备好接管你的任务',
    'console.empty.description': '提交指令后，左侧将记录历史会话，右侧展示计划、工具调用与输出结果。',
    'console.input.placeholder.active': '继续对话，输入新的需求…',
    'console.input.placeholder.idle': '请输入你想完成的任务或问题…',
    'console.input.hotkeyHint': '按 Enter 发送 · Shift+Enter 换行',
    'console.toast.taskFailed': '任务执行失败',
    'connection.connected': '已连接',
    'connection.reconnecting': '重新连接中…（第 {attempt} 次）',
    'connection.disconnected': '已断开',
    'connection.reconnect': '重新连接',
    'timeline.card.title': '执行时间线',
    'timeline.card.subtitle': '追踪每一步研究进度',
    'timeline.card.toolsUsed': '使用的工具：',
    'timeline.card.tokensUsed': '消耗 Token：',
    'timeline.card.error': '错误信息：',
    'timeline.card.expand': '展开详情',
    'timeline.card.collapse': '收起详情',
    'timeline.card.badge.pending': '待执行',
    'timeline.card.badge.active': '执行中',
    'timeline.card.badge.complete': '已完成',
    'timeline.card.badge.error': '失败',
    'plan.title': '研究计划',
    'plan.caption.default': '审核并批准后开始执行',
    'plan.caption.readonly': '已批准的计划',
    'plan.collapse': '收起计划',
    'plan.expand': '展开计划',
    'plan.goal.label': '目标：',
    'plan.steps.label': '计划步骤（{count}）：',
    'plan.iterations': '迭代次数：',
    'plan.tools': '工具：',
    'plan.tools.more': '+{count} 项',
    'plan.actions.saveChanges': '保存修改',
    'plan.actions.cancel': '取消',
    'plan.actions.approve': '批准并开始',
    'plan.actions.modify': '调整计划',
    'plan.actions.reject': '拒绝计划',
    'plan.reject.reasonLabel': '拒绝原因',
    'plan.reject.placeholder': '为什么该计划无法执行？',
    'plan.reject.confirm': '确认拒绝',
    'plan.reject.cancel': '取消',
    'plan.edit.goal': '编辑目标',
    'plan.edit.stepLabel': '编辑步骤 {index}',
    'plan.move.up': '上移步骤 {index}',
    'plan.move.down': '下移步骤 {index}',
    'task.submit.title.running': '执行中…',
    'task.submit.title.default': '回车提交',
    'task.submit.running': '执行中',
    'task.submit.label': '发送',
    'task.input.ariaLabel': '任务输入框',
    'tool.status.failed': '失败',
    'tool.status.completed': '完成',
    'tool.toggle.expand': '展开输出',
    'tool.toggle.collapse': '收起输出',
    'tool.toggle.length': ' · {count} 字符',
    'tool.section.parameters': '参数',
    'tool.section.error': '错误',
    'tool.section.output': '输出',
    'sessions.archiveLabel': '会话归档',
    'sessions.title': '历史会话管理',
    'sessions.description': '查看、回溯并重新打开 ALEX 的自动化工作流。',
    'sessions.newConversation': '新建对话',
  },
} as const;

export type TranslationKey = keyof typeof translations.en;

type TranslationParams = Record<string, string | number>;

interface LanguageContextValue {
  language: Language;
  setLanguage: (language: Language) => void;
  t: (key: TranslationKey, params?: TranslationParams) => string;
}

const LanguageContext = createContext<LanguageContextValue | null>(null);

function format(template: string, params?: TranslationParams) {
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (_, token: string) => {
    const value = params[token];
    return value !== undefined ? String(value) : `{${token}}`;
  });
}

export function LanguageProvider({ children }: { children: React.ReactNode }) {
  const [language, setLanguage] = useState<Language>('en');

  useEffect(() => {
    if (isBrowser()) {
      document.documentElement.lang = language;
    }
  }, [language]);

  const translate = useCallback(
    (key: TranslationKey, params?: TranslationParams) => {
      const dictionary = translations[language];
      const fallback = translations.en;
      const template = (dictionary[key] ?? fallback[key]) as string;
      return format(template, params);
    },
    [language]
  );

  const value = useMemo<LanguageContextValue>(
    () => ({
      language,
      setLanguage,
      t: translate,
    }),
    [language, translate]
  );

  return <LanguageContext.Provider value={value}>{children}</LanguageContext.Provider>;
}

export function useI18n() {
  const context = useContext(LanguageContext);
  if (!context) {
    throw new Error('useI18n must be used within a LanguageProvider');
  }
  return context;
}

export function useTranslation() {
  const { t } = useI18n();
  return t;
}

export const supportedLanguages = [
  {
    code: 'en' as const,
    labelKey: 'language.option.en' as TranslationKey,
    shortKey: 'language.option.en.short' as TranslationKey,
  },
  {
    code: 'zh' as const,
    labelKey: 'language.option.zh' as TranslationKey,
    shortKey: 'language.option.zh.short' as TranslationKey,
  },
];
