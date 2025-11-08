import { describe, expect, it, beforeEach, afterEach, vi } from 'vitest';

process.env.NEXT_PUBLIC_SANDBOX_VIEWER_URL = 'about:blank';
import { act, render, screen, fireEvent, waitFor } from '@testing-library/react';
import ArticleWorkbenchPage from '../workbench/article/page';
import ImageWorkbenchPage from '../workbench/image/page';
import CodeWorkbenchPage from '../workbench/code/page';
import { WorkTypeCard } from '../workbench/components/WorkTypeCard';
import { FileText } from 'lucide-react';
import {
  generateArticleInsights,
  generateImageConcepts,
  generateWebBlueprint,
  generateCodePlan,
  saveArticleDraft,
  listArticleDrafts,
  deleteArticleDraft,
} from '@/lib/api';

vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('../../lib/api')>('../../lib/api');
  return {
    ...actual,
    generateArticleInsights: vi.fn().mockResolvedValue({
      summary: 'AI 摘要',
      key_points: ['要点一'],
      suggestions: ['建议一'],
      citations: [],
      illustrations: [
        {
          paragraph_summary: '引言段落',
          image_idea: '展示文章主题的视觉概念',
          prompt: 'intro paragraph illustration, warm light',
          keywords: ['intro'],
          craft_id: 'craft-image-1',
          image_url: 'https://example.com/craft-image-1.png',
          media_type: 'image/png',
          name: 'article-illustration.png',
        },
      ],
      session_id: 'session-test',
      task_id: 'task-test',
    }),
    generateImageConcepts: vi.fn().mockResolvedValue({
      concepts: [
        {
          title: '霓虹雨夜',
          prompt: 'futuristic neon city, rain reflections',
          style_notes: ['强调冷暖对比'],
          aspect_ratio: '16:9',
        },
      ],
      session_id: 'session-image',
      task_id: 'task-image',
    }),
    generateWebBlueprint: vi.fn().mockResolvedValue({
      blueprint: {
        page_title: 'Alex 工作台登陆页',
        summary: '展示 Alex 工作台的价值与核心模块',
        sections: [
          {
            title: '首屏价值',
            purpose: '传达核心主张',
            components: ['标题', '副标题'],
            copy_suggestions: ['与 AI 协同的多模态创作平台'],
          },
          {
            title: '功能亮点',
            purpose: '展示关键能力',
            components: ['三列图文'],
            copy_suggestions: ['文章、图片、代码一站式支持'],
          },
        ],
        call_to_actions: [
          {
            label: '立即体验',
            destination: '/signup',
            variant: 'primary',
          },
        ],
        seo_keywords: ['AI 工作台'],
      },
      session_id: 'session-web',
      task_id: 'task-web',
    }),
    generateCodePlan: vi.fn().mockResolvedValue({
      plan: {
        service_name: 'Demo Service',
        summary: '自动化演示微服务蓝图',
        language: 'Python',
        runtime: 'Python + FastAPI',
        architecture: ['事件驱动处理', '容器化部署'],
        components: [
          {
            name: 'API 层',
            responsibility: '提供 RESTful 接口',
            tech_notes: ['使用 FastAPI 定义路由'],
          },
        ],
        api_endpoints: [
          {
            method: 'post',
            path: '/api/demo',
            description: '创建示例资源',
            request_schema: '{"name":string}',
            response_schema: '{"id":string}',
          },
        ],
        dev_tasks: ['补充单元测试'],
        operations: ['暴露 Prometheus 指标'],
        testing: ['运行 pytest'],
      },
      session_id: 'session-code',
      task_id: 'task-code',
    }),
    saveArticleDraft: vi.fn().mockResolvedValue({
      session_id: 'session-test',
      craft: {
        id: 'craft-1',
        session_id: 'session-test',
        user_id: 'user-1',
        name: '文章草稿.html',
        media_type: 'text/html',
        storage_key: 'user-1/craft-1.html',
        created_at: new Date().toISOString(),
      },
    }),
    listArticleDrafts: vi.fn().mockResolvedValue({
      drafts: [
        {
          craft: {
            id: 'craft-1',
            session_id: 'session-test',
            user_id: 'user-1',
            name: '文章草稿.html',
            media_type: 'text/html',
            storage_key: 'user-1/craft-1.html',
            created_at: new Date().toISOString(),
          },
          download_url: 'https://example.com/craft-1',
        },
      ],
    }),
    deleteArticleDraft: vi.fn().mockResolvedValue(undefined),
  };
});

let WebWorkbenchPage: (typeof import('../workbench/web/page'))['default'];

describe('WorkTypeCard', () => {
  it('renders title and navigates to href', () => {
    render(
      <WorkTypeCard
        href="/workbench/article"
        title="文章创作"
        description="描述"
        icon={<FileText aria-hidden />}
      />
    );

    expect(screen.getByRole('link', { name: /文章创作/ })).toHaveAttribute('href', '/workbench/article');
  });
});

describe('ArticleWorkbenchPage', () => {
  let fetchSpy: ReturnType<typeof vi.spyOn> | null = null;
  const originalClipboard = navigator.clipboard;
  const clipboardMock = {
    writeText: vi.fn().mockResolvedValue(undefined),
  } as Pick<Clipboard, 'writeText'>;

  beforeEach(() => {
    vi.useFakeTimers();
    Object.defineProperty(window, 'prompt', {
      value: vi.fn(),
      configurable: true,
    });
    Object.defineProperty(document, 'execCommand', {
      value: vi.fn(),
      configurable: true,
    });
    clipboardMock.writeText.mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: clipboardMock,
      configurable: true,
    });
    fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue({
      ok: true,
      text: () => Promise.resolve('<p>加载的草稿</p>'),
    } as Response);
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    fetchSpy?.mockRestore();
    fetchSpy = null;
    vi.clearAllMocks();
    if (originalClipboard) {
      Object.defineProperty(navigator, 'clipboard', {
        value: originalClipboard,
        configurable: true,
      });
    } else {
      Reflect.deleteProperty(navigator as unknown as Record<string, unknown>, 'clipboard');
    }
  });

  it('shows article studio header, sandbox iframe, and loads insights', async () => {
    render(<ArticleWorkbenchPage />);

    expect(screen.getByText('所见即所得的文章工作台')).toBeInTheDocument();
    expect(screen.getByTitle('Agent Sandbox Viewer')).toBeInTheDocument();

    await act(async () => {
      vi.advanceTimersByTime(1500);
      await Promise.resolve();
    });

    expect(generateArticleInsights).toHaveBeenCalled();

    await act(async () => {
      await Promise.resolve();
    });

    expect(screen.getByText('AI 摘要')).toBeInTheDocument();
    expect(screen.getByText('展示文章主题的视觉概念')).toBeInTheDocument();
    expect(screen.getByAltText('展示文章主题的视觉概念')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '查看 / 下载' })).toBeInTheDocument();
  });

  it('allows saving the draft to crafts', async () => {
    render(<ArticleWorkbenchPage />);

    const saveButton = screen.getByRole('button', { name: '保存到 Crafts' });
    await act(async () => {
      fireEvent.click(saveButton);
      await Promise.resolve();
    });

    expect(saveArticleDraft).toHaveBeenCalled();
    expect(screen.getByText(/已保存到 Crafts/)).toBeInTheDocument();
  });

  it('lists saved drafts and loads selected draft content', async () => {
    vi.useRealTimers();
    render(<ArticleWorkbenchPage />);

    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(listArticleDrafts).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(screen.getByText('文章草稿.html')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: '载入' })).toBeInTheDocument();
    });

    const loadButton = screen.getByRole('button', { name: '载入' });
    await act(async () => {
      fireEvent.click(loadButton);
      await Promise.resolve();
    });

    expect(global.fetch).toHaveBeenCalledWith('https://example.com/craft-1');
    expect(screen.getByText(/已载入草稿/)).toBeInTheDocument();
  }, 10000);

  it('allows deleting a saved draft from history', async () => {
    vi.useRealTimers();
    render(<ArticleWorkbenchPage />);

    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(listArticleDrafts).toHaveBeenCalled();
    });

    const deleteButton = await screen.findByRole('button', { name: '删除' });

    await act(async () => {
      fireEvent.click(deleteButton);
      await Promise.resolve();
    });

    expect(deleteArticleDraft).toHaveBeenCalledWith('craft-1');
    await waitFor(() => {
      expect(screen.getByText(/已删除草稿/)).toBeInTheDocument();
    });
  }, 10000);

});

describe('ImageWorkbenchPage', () => {
  it('submits brief and renders generated concepts', async () => {
    render(<ImageWorkbenchPage />);

    const briefInput = screen.getByPlaceholderText('例如：未来城市主题海报，突出霓虹灯与雨夜街景');
    const submitButton = screen.getByRole('button', { name: '生成视觉方向' });

    fireEvent.change(briefInput, { target: { value: '未来城市夜景海报' } });
    fireEvent.click(submitButton);

    expect(generateImageConcepts).toHaveBeenCalled();

    await waitFor(() => {
      expect(screen.getByText('霓虹雨夜')).toBeInTheDocument();
      expect(screen.getByText('futuristic neon city, rain reflections')).toBeInTheDocument();
      expect(screen.getByText('Session: session-image')).toBeInTheDocument();
    });
  });

  it('copies prompt to clipboard', async () => {
    render(<ImageWorkbenchPage />);

    fireEvent.change(
      screen.getByPlaceholderText('例如：未来城市主题海报，突出霓虹灯与雨夜街景'),
      { target: { value: '霓虹城市' } }
    );

    fireEvent.click(screen.getByRole('button', { name: '生成视觉方向' }));

    await waitFor(() => {
      expect(screen.getByText('霓虹雨夜')).toBeInTheDocument();
    });

    const copyButton = screen.getByRole('button', { name: '复制提示词' });
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: '已复制' })).toBeInTheDocument();
    });
  });
});

describe('WebWorkbenchPage', () => {
  let originalIframeSrc: PropertyDescriptor | undefined;
  let originalIframeSetAttribute: ((this: HTMLIFrameElement, qualifiedName: string, value: string) => void) | undefined;

  beforeAll(async () => {
    vi.resetModules();
    process.env.NEXT_PUBLIC_SANDBOX_VIEWER_URL = 'about:blank';
    originalIframeSrc = Object.getOwnPropertyDescriptor(HTMLIFrameElement.prototype, 'src');
    Object.defineProperty(HTMLIFrameElement.prototype, 'src', {
      configurable: true,
      enumerable: true,
      get() {
        return this.getAttribute('data-src') ?? '';
      },
      set(value) {
        this.setAttribute('data-src', value);
      },
    });
    originalIframeSetAttribute = HTMLIFrameElement.prototype.setAttribute;
    HTMLIFrameElement.prototype.setAttribute = function (name: string, value: string) {
      if (name === 'src') {
        if (originalIframeSetAttribute) {
          originalIframeSetAttribute.call(this, 'data-src', value);
        }
        return;
      }
      return originalIframeSetAttribute?.call(this, name, value);
    };
    WebWorkbenchPage = (await import('../workbench/web/page')).default;
  });

  afterAll(() => {
    if (originalIframeSrc) {
      Object.defineProperty(HTMLIFrameElement.prototype, 'src', originalIframeSrc);
    }
    if (originalIframeSetAttribute) {
      HTMLIFrameElement.prototype.setAttribute = originalIframeSetAttribute;
    }
  });

  const originalClipboard = navigator.clipboard;
  const clipboardMock = {
    writeText: vi.fn().mockResolvedValue(undefined),
  } as Pick<Clipboard, 'writeText'>;
  let fetchSpy: ReturnType<typeof vi.spyOn> | null = null;

  beforeEach(() => {
    fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(''),
    } as Response);
    Object.defineProperty(navigator, 'clipboard', {
      value: clipboardMock,
      configurable: true,
  });
});

describe('CodeWorkbenchPage', () => {
  const originalClipboard = navigator.clipboard;
  const clipboardMock = {
    writeText: vi.fn().mockResolvedValue(undefined),
  } as Pick<Clipboard, 'writeText'>;

  beforeEach(() => {
    Object.defineProperty(navigator, 'clipboard', {
      value: clipboardMock,
      configurable: true,
    });
  });

  afterEach(() => {
    clipboardMock.writeText.mockReset();
    if (originalClipboard) {
      Object.defineProperty(navigator, 'clipboard', {
        value: originalClipboard,
        configurable: true,
      });
    } else {
      Reflect.deleteProperty(navigator as unknown as Record<string, unknown>, 'clipboard');
    }
  });

  it('submits form and renders code plan', async () => {
    render(<CodeWorkbenchPage />);

    fireEvent.change(screen.getByPlaceholderText('例如：订单状态聚合服务'), {
      target: { value: '通知微服务' },
    });
    fireEvent.change(screen.getByPlaceholderText('说明业务背景、需要解决的问题以及成功标准'), {
      target: { value: '整合事件并推送通知' },
    });
    fireEvent.change(screen.getByPlaceholderText('例如：Go + chi / Python + FastAPI'), {
      target: { value: 'Python + FastAPI' },
    });
    fireEvent.change(screen.getByLabelText('关键功能（每行一项，可选）'), {
      target: { value: '自动触发通知' },
    });

    fireEvent.click(screen.getByRole('button', { name: '生成微服务蓝图' }));

    expect(generateCodePlan).toHaveBeenCalled();

    await waitFor(() => {
      expect(screen.getByText('自动化演示微服务蓝图')).toBeInTheDocument();
    });

    expect(screen.getByText(/Python \+ FastAPI/)).toBeInTheDocument();
    expect(screen.getByText('创建示例资源')).toBeInTheDocument();
    expect(screen.getByText(/session-code/)).toBeInTheDocument();
    expect(generateCodePlan).toHaveBeenCalledWith(
      expect.objectContaining({
        service_name: '通知微服务',
        features: ['自动触发通知'],
      })
    );
  });

  it('copies endpoint definition to clipboard', async () => {
    render(<CodeWorkbenchPage />);

    fireEvent.change(screen.getByPlaceholderText('例如：订单状态聚合服务'), {
      target: { value: '通知微服务' },
    });
    fireEvent.change(screen.getByPlaceholderText('说明业务背景、需要解决的问题以及成功标准'), {
      target: { value: '整合事件并推送通知' },
    });

    fireEvent.click(screen.getByRole('button', { name: '生成微服务蓝图' }));

    await waitFor(() => {
      expect(screen.getByText('创建示例资源')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: '复制' }));

    await waitFor(() => {
      expect(clipboardMock.writeText).toHaveBeenCalledWith('POST /api/demo');
      expect(screen.getByRole('button', { name: '已复制' })).toBeInTheDocument();
    });
  });
});

  afterEach(() => {
    vi.clearAllTimers();
    clipboardMock.writeText.mockReset();
    fetchSpy?.mockRestore();
    fetchSpy = null;
    if (originalClipboard) {
      Object.defineProperty(navigator, 'clipboard', {
        value: originalClipboard,
        configurable: true,
      });
    } else {
      Reflect.deleteProperty(navigator as unknown as Record<string, unknown>, 'clipboard');
    }
  });

  it('submits goal and renders blueprint with copy support', async () => {
    render(<WebWorkbenchPage />);

    fireEvent.change(screen.getByPlaceholderText(/预热落地页/), {
      target: { value: '设计 Alex 工作台登陆页' },
    });

    fireEvent.click(screen.getByRole('button', { name: '生成页面蓝图' }));

    await waitFor(() => {
      expect(generateWebBlueprint).toHaveBeenCalled();
    });

    expect(await screen.findByText('首屏价值')).toBeInTheDocument();
    expect(screen.getByText('功能亮点')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '复制 JSON' }));

    await waitFor(() => {
      expect(clipboardMock.writeText).toHaveBeenCalled();
    });
  }, 20000);
});
