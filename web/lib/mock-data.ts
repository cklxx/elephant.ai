import type {
  ArticleCraftListResponse,
  ArticleCraftResponse,
  ArticleDraftSummary,
  ArticleInsightResponse,
  Craft,
  CraftDownloadResponse,
  CraftListResponse,
  ImageConceptResponse,
  Session,
  SessionDetailsResponse,
  SessionListResponse,
  TaskStatusResponse,
  WebBlueprintResponse,
  CodePlanResponse,
  GenerateWebBlueprintPayload,
  GenerateImageConceptsPayload,
  GenerateCodePlanPayload,
  SaveArticleDraftPayload,
} from './types';

const USER_ID = 'user-demo';

const BASE_TIME = new Date('2024-11-20T08:30:00.000Z').getTime();
const MINUTE = 60 * 1000;

function isoMinutesAgo(minutes: number) {
  return new Date(BASE_TIME - minutes * MINUTE).toISOString();
}

function stripHtml(payload: string) {
  return payload.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
}

function createMockId(prefix: string) {
  return `${prefix}-${Math.random().toString(36).slice(2, 10)}`;
}

interface MockState {
  sessions: Session[];
  tasks: Record<string, TaskStatusResponse[]>;
  crafts: Craft[];
  craftDownloads: Record<string, string>;
  articleDrafts: ArticleDraftSummary[];
}

const initialSessions: Session[] = [
  {
    id: 'sess-brand-landing',
    created_at: isoMinutesAgo(540),
    updated_at: isoMinutesAgo(35),
    task_count: 6,
    last_task: '更新产品故事线',
  },
  {
    id: 'sess-ai-report',
    created_at: isoMinutesAgo(320),
    updated_at: isoMinutesAgo(10),
    task_count: 4,
    last_task: '整理 AI 行业周报',
  },
];

const initialTasks: Record<string, TaskStatusResponse[]> = {
  'sess-brand-landing': [
    {
      task_id: 'task-landing-outline',
      session_id: 'sess-brand-landing',
      status: 'completed',
      created_at: isoMinutesAgo(120),
      completed_at: isoMinutesAgo(118),
      error: undefined,
      parent_task_id: undefined,
    },
    {
      task_id: 'task-landing-hero',
      session_id: 'sess-brand-landing',
      status: 'completed',
      created_at: isoMinutesAgo(90),
      completed_at: isoMinutesAgo(87),
      error: undefined,
      parent_task_id: undefined,
    },
  ],
  'sess-ai-report': [
    {
      task_id: 'task-weekly-digest',
      session_id: 'sess-ai-report',
      status: 'completed',
      created_at: isoMinutesAgo(60),
      completed_at: isoMinutesAgo(55),
      error: undefined,
      parent_task_id: undefined,
    },
    {
      task_id: 'task-qa-followup',
      session_id: 'sess-ai-report',
      status: 'running',
      created_at: isoMinutesAgo(12),
      completed_at: undefined,
      error: undefined,
      parent_task_id: undefined,
    },
  ],
};

const initialCrafts: Craft[] = [
  {
    id: 'craft-article-brand-story',
    session_id: 'sess-brand-landing',
    user_id: USER_ID,
    name: '品牌故事落地页草稿',
    media_type: 'text/html',
    description: '首屏文案与关键要点',
    source: 'workbench.article',
    size: 4200,
    checksum: 'mock-checksum-1',
    storage_key: 'mock/crafts/article-brand-story.html',
    created_at: isoMinutesAgo(70),
  },
  {
    id: 'craft-image-hero-concepts',
    session_id: 'sess-brand-landing',
    user_id: USER_ID,
    name: 'Hero Banner 灵感合集',
    media_type: 'application/json',
    description: '包含 4 个 AI 生成的视觉方向',
    source: 'workbench.image',
    size: 2048,
    checksum: 'mock-checksum-2',
    storage_key: 'mock/crafts/image-hero-concepts.json',
    created_at: isoMinutesAgo(65),
  },
  {
    id: 'craft-web-wireframe',
    session_id: 'sess-brand-landing',
    user_id: USER_ID,
    name: '落地页模块结构',
    media_type: 'application/json',
    description: '包含页面区块与 CTA 配置',
    source: 'workbench.web',
    size: 3584,
    checksum: 'mock-checksum-3',
    storage_key: 'mock/crafts/web-wireframe.json',
    created_at: isoMinutesAgo(50),
  },
  {
    id: 'craft-code-service-plan',
    session_id: 'sess-ai-report',
    user_id: USER_ID,
    name: '周报自动化服务蓝图',
    media_type: 'application/json',
    description: '后端 API 规划与队列策略',
    source: 'workbench.code',
    size: 5120,
    checksum: 'mock-checksum-4',
    storage_key: 'mock/crafts/code-service-plan.json',
    created_at: isoMinutesAgo(30),
  },
];

const initialDrafts: ArticleDraftSummary[] = [
  {
    craft: initialCrafts[0],
    download_url: 'https://files.mock.alex/crafts/article-brand-story.html',
  },
];

const mockState: MockState = {
  sessions: [...initialSessions],
  tasks: { ...initialTasks },
  crafts: [...initialCrafts],
  craftDownloads: {
    'craft-article-brand-story': 'https://files.mock.alex/crafts/article-brand-story.html',
    'craft-image-hero-concepts': 'https://files.mock.alex/crafts/image-hero-concepts.json',
    'craft-web-wireframe': 'https://files.mock.alex/crafts/web-wireframe.json',
    'craft-code-service-plan': 'https://files.mock.alex/crafts/code-service-plan.json',
  },
  articleDrafts: [...initialDrafts],
};

export function getMockSessionList(): SessionListResponse {
  return {
    sessions: [...mockState.sessions],
    total: mockState.sessions.length,
  };
}

export function getMockSessionDetails(sessionId: string): SessionDetailsResponse {
  const session = mockState.sessions.find((item) => item.id === sessionId);
  if (!session) {
    throw new Error(`Mock session ${sessionId} not found`);
  }

  return {
    session,
    tasks: [...(mockState.tasks[sessionId] ?? [])],
  };
}

export function deleteMockSession(sessionId: string): void {
  mockState.sessions = mockState.sessions.filter((item) => item.id !== sessionId);
  delete mockState.tasks[sessionId];
}

export function forkMockSession(sessionId: string): { new_session_id: string } {
  const original = mockState.sessions.find((item) => item.id === sessionId);
  const newId = createMockId('sess');
  const createdAt = new Date().toISOString();

  mockState.sessions.unshift({
    id: newId,
    created_at: createdAt,
    updated_at: createdAt,
    task_count: original ? original.task_count : 0,
    last_task: original?.last_task,
  });

  if (original) {
    mockState.tasks[newId] = (mockState.tasks[sessionId] ?? []).map((task) => ({
      ...task,
      session_id: newId,
      task_id: createMockId(task.task_id ?? 'task'),
    }));
  } else {
    mockState.tasks[newId] = [];
  }

  return { new_session_id: newId };
}

export function getMockCraftList(): CraftListResponse {
  return {
    crafts: [...mockState.crafts],
  };
}

export function deleteMockCraft(craftId: string): void {
  mockState.crafts = mockState.crafts.filter((craft) => craft.id !== craftId);
  mockState.articleDrafts = mockState.articleDrafts.filter(
    (draft) => draft.craft.id !== craftId
  );
  delete mockState.craftDownloads[craftId];
}

export function getMockCraftDownloadUrl(craftId: string): CraftDownloadResponse {
  const url = mockState.craftDownloads[craftId];
  if (!url) {
    return { url: `https://files.mock.alex/crafts/${craftId}.json` };
  }
  return { url };
}

export function getMockArticleDrafts(): ArticleCraftListResponse {
  return {
    drafts: [...mockState.articleDrafts],
  };
}

export function deleteMockArticleDraft(craftId: string): void {
  deleteMockCraft(craftId);
}

export function saveMockArticleDraft(
  payload: SaveArticleDraftPayload
): ArticleCraftResponse {
  const sessionId = payload.session_id ?? 'sess-article-draft';
  const now = new Date().toISOString();
  const title = payload.title?.trim() || '文章草稿';
  const craftId = createMockId('craft-article');
  const craft: Craft = {
    id: craftId,
    session_id: sessionId,
    user_id: USER_ID,
    name: title,
    media_type: 'text/html',
    description: payload.summary || stripHtml(payload.content).slice(0, 120),
    source: 'workbench.article',
    size: payload.content.length,
    checksum: `checksum-${craftId}`,
    storage_key: `mock/crafts/${craftId}.html`,
    created_at: now,
  };

  const downloadUrl = `https://files.mock.alex/crafts/${craftId}.html`;
  mockState.crafts.unshift(craft);
  mockState.articleDrafts.unshift({ craft, download_url: downloadUrl });
  mockState.craftDownloads[craftId] = downloadUrl;

  if (!mockState.sessions.some((item) => item.id === sessionId)) {
    mockState.sessions.unshift({
      id: sessionId,
      created_at: now,
      updated_at: now,
      task_count: 1,
      last_task: '保存文章草稿',
    });
    mockState.tasks[sessionId] = [];
  }

  return {
    craft,
    session_id: sessionId,
  };
}

export function buildMockArticleInsights(content: string): ArticleInsightResponse {
  const plain = stripHtml(content);
  const summary = plain
    ? `根据当前草稿整理出关键脉络：${plain.slice(0, 80)}${plain.length > 80 ? '…' : ''}`
    : '当前草稿为空，建议先写出大纲或要点。';

  const keyBase = plain || '草稿';

  return {
    summary,
    key_points: [
      `${keyBase.slice(0, 12)}：聚焦读者的核心关切`,
      '补充两到三个真实案例或数据引用',
      '使用短段落与副标题提升可读性',
    ],
    suggestions: [
      '为每个段落加上小结，帮助读者快速扫读',
      '在结尾加入明确的行动号召或延伸阅读链接',
      '引用 crafts 中的视觉素材，增强文章吸引力',
    ],
    citations: [
      {
        title: '行业白皮书：品牌增长策略 2024',
        source: 'Insight Research',
        url: 'https://example.com/reports/brand-growth-2024',
        snippet: '总结新兴品牌在数字渠道中取得成功的三种方式。',
      },
      {
        title: '用户调研访谈要点',
        url: 'https://example.com/research/interviews',
        snippet: '客户最关注 onboarding 体验与首次价值实现。',
      },
    ],
    illustrations: [
      {
        paragraph_summary: '开篇段落：引出主题与读者痛点',
        image_idea: '柔和灯光下的创作者在桌前与 AI 协作的场景',
        prompt:
          'modern creative studio, person collaborating with holographic ai interface, warm desk lamp, cinematic lighting, ultra realistic',
        keywords: ['creative studio', 'holographic UI', 'warm lighting'],
        craft_id: 'craft-img-1',
        image_url: 'https://example.com/crafts/craft-img-1.png',
        media_type: 'image/png',
        name: 'collaboration.png',
      },
      {
        paragraph_summary: '案例段：展示成功品牌的合作案例',
        image_idea: '品牌团队围绕大屏讨论数据面板，突出增长指标',
        prompt:
          'diverse marketing team reviewing large data dashboard, growth metrics highlighted, futuristic office, depth of field',
        keywords: ['data dashboard', 'team collaboration'],
        craft_id: 'craft-img-2',
        image_url: 'https://example.com/crafts/craft-img-2.png',
        media_type: 'image/png',
        name: 'dashboard.png',
      },
      {
        paragraph_summary: '收尾段：给出可执行的行动清单',
        image_idea: '任务列表与日程排期的桌面平铺拍摄，强调执行力',
        prompt:
          'flat lay of productivity planner, checklist, calendar stickers, pastel color palette, soft daylight, high detail',
        keywords: ['flat lay', 'productivity planner'],
        craft_id: 'craft-img-3',
        image_url: 'https://example.com/crafts/craft-img-3.png',
        media_type: 'image/png',
        name: 'planner.png',
      },
    ],
    session_id: 'sess-brand-landing',
    task_id: 'task-landing-outline',
  };
}

export function buildMockImageConcepts(
  payload: GenerateImageConceptsPayload
): ImageConceptResponse {
  const base = payload.brief.trim() || '品牌视觉灵感';
  const references = (payload.references ?? []).filter(Boolean);

  return {
    concepts: [
      {
        title: `${base} · 情绪主视觉`,
        prompt: `${base}，柔和霓虹光，强调人与技术的协作`,
        style_notes: ['使用蓝紫渐变背景', '加入细腻的玻璃态 UI 元素'],
        aspect_ratio: '16:9',
        mood: 'futuristic calm',
      },
      {
        title: `${base} · 动态分镜`,
        prompt: `${base}，动态图层，突出产品流程`,
        style_notes: ['动态流线切片', '对比色突出 CTA'],
        aspect_ratio: '4:5',
        seed_hint: 'seed-42',
      },
      {
        title: `${base} · 质感摄影`,
        prompt: `${base}，实拍质感，浅景深`,
        style_notes: ['暖色系照明', '加入真实使用场景'],
        aspect_ratio: '3:2',
      },
    ],
    session_id: 'sess-brand-landing',
    task_id: 'task-landing-hero',
  };
}

export function buildMockWebBlueprint(
  payload: GenerateWebBlueprintPayload
): WebBlueprintResponse {
  const goal = payload.goal.trim() || '全新落地页';

  return {
    blueprint: {
      page_title: goal,
      summary: `为「${goal}」提供三屏结构，突出价值主张、产品能力与信任背书。`,
      sections: [
        {
          title: '首屏价值主张',
          purpose: '立即说明产品差异化优势',
          components: ['主标题 + 副标题', '核心 CTA 按钮', '关键数据徽章'],
          copy_suggestions: [
            '一句话描述最终价值，例如「5 分钟搭建你的多渠道工作台」',
            '副标题突出效率、智能与协作',
          ],
        },
        {
          title: '功能流程演示',
          purpose: '展示产品如何解决用户痛点',
          components: ['三步流程卡片', '演示视频或沙箱截图'],
        },
        {
          title: '社会证明与资源',
          purpose: '增强信任与转化',
          components: ['客户引言', '媒体报道', '下载白皮书按钮'],
        },
      ],
      call_to_actions: [
        {
          label: '预约演示',
          destination: '#demo',
          messaging: '与解决方案专家快速对接',
        },
        {
          label: '试用工作台',
          destination: '#signup',
          variant: 'secondary',
        },
      ],
      seo_keywords: ['AI 工作台', '智能协作平台', goal],
    },
    session_id: 'sess-brand-landing',
    task_id: 'task-web-blueprint',
  };
}

export function buildMockCodePlan(
  payload: GenerateCodePlanPayload
): CodePlanResponse {
  const serviceName = payload.service_name.trim() || '代码微服务 Demo';

  return {
    plan: {
      service_name: serviceName,
      objective:
        payload.objective || '为业务团队提供可快速验证的 API 与前端示例',
      architecture: 'NestJS + PostgreSQL + Redis 队列',
      components: [
        {
          name: 'API Gateway',
          responsibility: '统一鉴权与路由转发',
          tech_notes: ['基于 Fastify 适配器', '支持 JWT 与临时令牌'],
        },
        {
          name: 'Job Processor',
          responsibility: '消费生成任务并同步写入 crafts',
          tech_notes: ['BullMQ 队列', '自动重试与告警'],
        },
        {
          name: 'Static Preview',
          responsibility: '托管前端 Demo 与截图',
        },
      ],
      endpoints: [
        {
          method: 'POST',
          path: '/api/v1/jobs',
          description: '提交生成请求并返回任务 ID',
          request_schema: '{ "prompt": string, "references"?: string[] }',
          response_schema: '{ "job_id": string }',
        },
        {
          method: 'GET',
          path: '/api/v1/jobs/:id',
          description: '查询任务状态与产物链接',
        },
      ],
      deployment: {
        environments: ['preview', 'production'],
        notes: ['使用 Docker Compose 在 sandbox 中演示', 'CI 中触发单元与合成测试'],
      },
    },
    session_id: 'sess-ai-report',
    task_id: 'task-code-plan',
  };
}
