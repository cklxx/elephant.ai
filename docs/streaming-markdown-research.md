# React LLM Streaming Markdown Rendering Research

## Recommended renderer
- **react-markdown**: Active, well-documented, and easy to extend with `remark`/`rehype` plugins. It renders partial Markdown progressively, so it works well with streaming token-by-token updates.
- **Plugins to enable**: `remark-gfm` for table/list/task support, `remark-breaks` for chat-style hard breaks, `rehype-raw` gated behind sanitization for trusted inline HTML, and `rehype-sanitize` with a minimal allowlist to prevent XSS when HTML passthrough is needed.
- **Code/highlighting**: pair with `rehype-highlight` or a custom renderer for code blocks to avoid blocking layout shifts and keep control over themes.

## Streaming best practices
- **Incremental updates**: keep the rendered Markdown in React state and append incoming LLM chunks to the buffer; `react-markdown` will re-render efficiently without blocking the UI.
- **Suspense-friendly**: wrap streaming sections in `React.Suspense` boundaries so backpressure from heavy components (e.g., embeds) does not freeze the rest of the UI.
- **Virtualization**: for long transcripts, combine `react-markdown` with windowing (e.g., `react-virtualized` or `react-virtuoso`) to keep DOM nodes bounded while still showing the newest streamed tokens.
- **Stability**: debounce scroll-to-bottom logic and memoize expensive renderers (code blocks, math) to avoid thrashing as new tokens arrive.

## Attachment handling during streaming
- **Placeholder-first rendering**: detect attachment placeholders (e.g., `[demo.mp4]`) as tokens arrive and swap them for typed React components (`VideoPreview`, image/lightbox, document/embed) instead of plain `<img>` tags. Preserve a map of attachment metadata so streamed replacements stay consistent.
- **Progressive hydration**: render lightweight poster/loading states first, then hydrate richer viewers when attachment metadata or preview assets finish fetching.
- **Security**: gate `data:` and remote URLs through a sanitizer/allowlist. For signed URLs, refresh tokens lazily when the component becomes visible (IntersectionObserver) to minimize failed loads during long streams.
- **Accessibility**: ensure inline replacements carry `aria-label`/`alt` text from attachment descriptions and keep keyboard focus traps for video/document modals intact even while the message is still streaming.

## Alternative options (trade-offs)
- **@uiw/react-markdown-preview**: good for static previews, but less flexible for incremental streaming/custom token handling compared to `react-markdown`.
- **marked + custom renderer**: lightweight and fast but requires more manual work to stay secure and to support streaming attachments; best when bundle size is critical.

## Integration outline
- Use `react-markdown` with the plugins above and pass a `components` map that routes attachment placeholders to the app's preview components.
- Maintain a per-message attachment lookup keyed by URI/placeholder so inline renders and gallery tiles share the same metadata and loading state.
- Add a small tokenizer that recognizes placeholders before Markdown parsing; this lets you render partial tokens immediately and upgrade them when the attachment payload arrives.

## 实施方案（Implementation Plan）
1) **数据管道与状态管理** — ✅ 已完成：`TaskCompleteCard` 中保留附件元数据 Map，支持占位符与图库复用。
   - 在消息模型中同时保存 Markdown 文本和附件元数据（URI、MIME、描述、占位符名）。
   - 维护 `placeholder → attachment detail` 与 `uri → attachment detail` 的双向 Map，方便 Markdown 渲染与外部预览复用。
   - 流式追加 LLM 片段时，仅更新消息缓冲区；附件元数据变更时通过独立状态触发占位符替换，避免整段重渲染。

2) **占位符解析与流式替换** — ✅ 已完成：内联 `img` 渲染器会识别占位符并切换到对应视频/文档预览组件。
   - 在接收流时用正则快速标记 `[...]` 占位符，并提前输出轻量占位节点（骨架屏/进度条）。
   - 在 Markdown 渲染层提供自定义 `img`/`a` 渲染器：
     - 若 `src` 命中附件 Map 且类型为视频/文档，替换为业务组件（如 `VideoPreview`、`ArtifactPreviewCard`）。
     - 其余场景保持原样 `<img>`，但继承 `alt` 文本与 `aria-label`。
   - 对未被 Markdown 内联引用的附件，在消息尾部渲染媒体图库，保证可发现性。

3) **安全与可访问性** — ⏳ 进行中：已列出白名单/ARIA 要求，待接入 `rehype-sanitize` 配置与懒加载签名 URL 刷新。
   - 对远程 URL 做协议/域名白名单校验；`data:` URL 仅允许受信内容类型。
   - `rehype-sanitize` 配置最小化允许列表，并为可交互附件组件添加键盘导航与 `aria` 文本。
   - 在需要刷新签名 URL 的附件上使用 `IntersectionObserver` 懒加载，失败时暴露重试按钮并上报日志。

4) **性能与回退** — ⏳ 进行中：虚拟滚动与懒加载策略待实装，当前保留占位符文本以确保可读性。
   - 代码块、数学公式等重组件使用 `React.Suspense` + lazy import；长列表结合虚拟滚动。
   - 在网络慢或附件缺失时保留占位符文本，降级为纯 Markdown 渲染以保证可读性。
   - 记录渲染耗时和失败率（如 Sentry breadcrumb + 性能埋点），为后续优化提供指标。
