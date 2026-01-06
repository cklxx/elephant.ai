package builtin

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/llm"
)

type miniAppHTML struct {
	llm ports.LLMClient
}

// NewMiniAppHTML creates a tool that assembles a single-file HTML mini-app from a short brief.
// The HTML body is synthesized with the configured LLM so the experience remains varied and
// prompt-driven, then wrapped in a resilient shell for static hosting.
func NewMiniAppHTML() ports.ToolExecutor {
	return NewMiniAppHTMLWithLLM(nil)
}

// NewMiniAppHTMLWithLLM allows injecting a specific LLM client (useful for tests).
func NewMiniAppHTMLWithLLM(client ports.LLMClient) ports.ToolExecutor {
	if client == nil {
		client = llm.NewMockClient()
	}
	return &miniAppHTML{llm: client}
}

func (t *miniAppHTML) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "miniapp_html",
		Version:  "1.0.0",
		Category: "web",
		Tags:     []string{"html", "miniapp", "game"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces: []string{"text/html"},
		},
	}
}

func (t *miniAppHTML) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "miniapp_html",
		Description: "Generate a single-file HTML mini-app or web mini-game ready for static hosting.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"title": {
					Type:        "string",
					Description: "Game or experience title to display in the header.",
				},
				"prompt": {
					Type:        "string",
					Description: "Creative brief for the mini-app: theme, goal, and interactions.",
				},
				"theme_color": {
					Type:        "string",
					Description: "Accent color (CSS value) for primary UI elements.",
				},
				"cta_label": {
					Type:        "string",
					Description: "Label for the primary action button (default: Start!).",
				},
			},
			Required: []string{"prompt"},
		},
	}
}

func (t *miniAppHTML) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	prompt := strings.TrimSpace(stringArg(call.Arguments, "prompt"))
	if prompt == "" {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("prompt is required")}, nil
	}

	title := strings.TrimSpace(stringArg(call.Arguments, "title"))
	if title == "" {
		title = "AI Mini App"
	}

	theme := strings.TrimSpace(stringArg(call.Arguments, "theme_color"))
	if theme == "" {
		theme = "#5B8DEF"
	}

	cta := strings.TrimSpace(stringArg(call.Arguments, "cta_label"))
	if cta == "" {
		cta = "Start!"
	}

	llmHTML := t.generateWithLLM(ctx, title, prompt, theme, cta)
	html := t.buildHTML(title, prompt, theme, cta, llmHTML)
	encoded := base64.StdEncoding.EncodeToString([]byte(html))

	attachment := ports.Attachment{
		Name:           fmt.Sprintf("%s.html", safeFilename(title)),
		MediaType:      "text/html",
		Data:           encoded,
		Description:    fmt.Sprintf("Mini-app generated for: %s", prompt),
		Kind:           "artifact",
		Format:         "html",
		PreviewProfile: "document.html",
		PreviewAssets: []ports.AttachmentPreviewAsset{
			{
				AssetID:     "live-preview",
				Label:       "HTML preview",
				MimeType:    "text/html",
				CDNURL:      fmt.Sprintf("data:text/html;base64,%s", encoded),
				PreviewType: "iframe",
			},
		},
	}

	content := "Generated single-file HTML mini-app. Save to static storage and open to play."

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     content,
		Attachments: map[string]ports.Attachment{attachment.Name: attachment},
		Metadata: map[string]any{
			"prefill_task": fmt.Sprintf("预览并改进小游戏《%s》：聚焦节奏、动效和易用性", title),
		},
	}, nil
}

func (t *miniAppHTML) generateWithLLM(ctx context.Context, title, prompt, theme, cta string) string {
	system := "You are an expert HTML5 mini-game designer. Produce compact, self-contained HTML that plays well on mobile and desktop. The game must be meme-forward and silly—lean into emojis, exaggerated reactions, and lightweight mechanics. Use inline scripts only; no external assets."
	user := fmt.Sprintf("Title: %s\nTheme color: %s\nCTA: %s\nBrief: %s\nReturn ONLY HTML. Focus on meme energy, quick laughs, and clear scoring feedback.", title, theme, cta, prompt)
	assistantPrefill := `<div class="meme-prefill">
  <p>😂 Meme-ready mini-game shell</p>
  <button aria-label="Start meme chaos">Start</button>
</div>`

	streaming, ok := ports.EnsureStreamingClient(t.llm).(ports.StreamingLLMClient)
	if !ok {
		return ""
	}

	const progressChunkMinChars = 256
	var progressBuffer strings.Builder
	var contentBuffer strings.Builder
	callbacks := ports.CompletionStreamCallbacks{
		OnContentDelta: func(delta ports.ContentDelta) {
			if delta.Delta != "" {
				contentBuffer.WriteString(delta.Delta)
				progressBuffer.WriteString(delta.Delta)
				if progressBuffer.Len() >= progressChunkMinChars {
					ports.EmitToolProgress(ctx, progressBuffer.String(), false)
					progressBuffer.Reset()
				}
			}
			if delta.Final {
				if progressBuffer.Len() > 0 {
					ports.EmitToolProgress(ctx, progressBuffer.String(), false)
					progressBuffer.Reset()
				}
				ports.EmitToolProgress(ctx, "", true)
			}
		},
	}

	resp, err := streaming.StreamComplete(ctx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
			{Role: "assistant", Content: assistantPrefill},
		},
		MaxTokens:   800,
		Temperature: 0.7,
	}, callbacks)
	if err != nil || resp == nil {
		return ""
	}
	trimmed := strings.TrimSpace(resp.Content)
	if trimmed == "" {
		trimmed = strings.TrimSpace(contentBuffer.String())
	}
	return trimmed
}

func (t *miniAppHTML) buildHTML(title, prompt, theme, cta, llmHTML string) string {
	if strings.Contains(strings.ToLower(llmHTML), "<html") {
		return llmHTML
	}

	aiBlock := "Mock LLM response"
	if llmHTML != "" {
		aiBlock = llmHTML
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>%s</title>
  <style>
    :root {
      --accent: %s;
      --text: #111827;
      --muted: #6b7280;
      --surface: #f3f4f6;
      --card: #ffffff;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Inter", "Segoe UI", system-ui, -apple-system, sans-serif;
      background: radial-gradient(circle at 20%% 20%%, #eef2ff 0%%, #f9fafb 40%%, #ffffff 70%%);
      color: var(--text);
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 24px;
    }
    .shell {
      width: min(1200px, 100%%);
      background: var(--card);
      border-radius: 28px;
      box-shadow: 0 30px 80px rgba(0,0,0,0.08);
      padding: 28px;
      border: 1px solid #e5e7eb;
    }
    header {
      display: flex;
      align-items: center;
      gap: 14px;
      margin-bottom: 12px;
    }
    .dot {
      width: 14px; height: 14px; border-radius: 50%%;
      background: var(--accent);
      box-shadow: 0 0 16px rgba(0,0,0,0.08);
    }
    .title {
      font-size: 24px;
      font-weight: 800;
      letter-spacing: -0.01em;
    }
    .prompt {
      color: var(--muted);
      font-size: 14px;
      margin-bottom: 18px;
      line-height: 1.5;
    }
    .grid {
      display: grid;
      grid-template-columns: 320px 1fr;
      gap: 18px;
    }
    .panel {
      border: 1px solid #e5e7eb;
      border-radius: 20px;
      padding: 16px;
      background: var(--surface);
    }
    .button {
      width: 100%%;
      border: none;
      border-radius: 14px;
      padding: 14px 16px;
      font-size: 16px;
      font-weight: 700;
      background: linear-gradient(135deg, var(--accent), #9f7aea);
      color: white;
      cursor: pointer;
      box-shadow: 0 15px 35px rgba(0,0,0,0.15);
      transition: transform 120ms ease, box-shadow 120ms ease;
    }
    .button:hover { transform: translateY(-1px); box-shadow: 0 18px 40px rgba(0,0,0,0.18); }
    .stats {
      display: flex;
      gap: 10px;
      margin-top: 12px;
    }
    .stat {
      flex: 1;
      padding: 12px;
      background: #fff;
      border-radius: 14px;
      border: 1px dashed #e5e7eb;
      text-align: center;
    }
    .stat .label { color: var(--muted); font-size: 12px; }
    .stat .value { font-size: 20px; font-weight: 800; }
    #arena {
      position: relative;
      min-height: 420px;
      background: #0f172a;
      border-radius: 18px;
      overflow: hidden;
      border: 1px solid #1e293b;
      box-shadow: inset 0 0 0 1px rgba(255,255,255,0.03);
    }
    #arena canvas { width: 100%%; height: 100%%; display: block; }
    #log {
      margin-top: 12px;
      height: 120px;
      overflow: auto;
      background: #0b1220;
      color: #cbd5e1;
      padding: 12px;
      border-radius: 12px;
      font-family: "JetBrains Mono", monospace;
      font-size: 13px;
      border: 1px solid #1f2a3d;
    }
    .pill {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      background: rgba(91, 141, 239, 0.12);
      color: #1d4ed8;
      padding: 6px 10px;
      border-radius: 999px;
      font-size: 12px;
      border: 1px solid rgba(91,141,239,0.25);
    }
  </style>
</head>
<body>
  <div class="shell">
    <header>
      <div class="dot"></div>
      <div>
        <div class="title">%s</div>
        <div class="prompt">%s</div>
      </div>
    </header>

    <div class="grid">
      <div class="panel">
        <button class="button" id="start">%s</button>
        <div class="stats">
          <div class="stat">
            <div class="label">Score</div>
            <div class="value" id="score">0</div>
          </div>
          <div class="stat">
            <div class="label">Combo</div>
            <div class="value" id="combo">x1</div>
          </div>
        </div>
        <div style="margin-top:14px; display:flex; gap:8px; flex-wrap:wrap;">
          <div class="pill">节奏 & 触发</div>
          <div class="pill">视觉引导</div>
          <div class="pill">触摸 & 键盘</div>
        </div>
      </div>

      <div class="panel" style="padding:0; background:#0f172a;">
        <div id="arena">
          <canvas id="canvas"></canvas>
        </div>
        <div id="log"></div>
        <div id="ai-block" style="padding:12px; color:#cbd5e1; font-family:'JetBrains Mono', monospace; font-size:13px; border-top:1px solid #1f2a3d;">
          %s
        </div>
      </div>
    </div>
  </div>

  <script>
    const canvas = document.getElementById("canvas");
    const ctx = canvas.getContext("2d");
    const logEl = document.getElementById("log");
    const scoreEl = document.getElementById("score");
    const comboEl = document.getElementById("combo");
    const startBtn = document.getElementById("start");

    let score = 0;
    let combo = 1;
    let active = false;
    let targets = [];

    function resize() {
      canvas.width = canvas.clientWidth * window.devicePixelRatio;
      canvas.height = canvas.clientHeight * window.devicePixelRatio;
      ctx.scale(window.devicePixelRatio, window.devicePixelRatio);
    }
    window.addEventListener("resize", resize);
    resize();

    function log(msg) {
      const time = new Date().toLocaleTimeString();
      logEl.innerHTML = "[" + time + "] " + msg + "<br>" + logEl.innerHTML;
    }

    function spawnTarget() {
      const size = 36 + Math.random() * 24;
      const x = 20 + Math.random() * (canvas.clientWidth - size - 20);
      const y = 20 + Math.random() * (canvas.clientHeight - size - 20);
      targets.push({ x, y, size, life: 2400 + Math.random()*800 });
    }

    function draw() {
      ctx.clearRect(0, 0, canvas.clientWidth, canvas.clientHeight);
      targets.forEach((t) => {
        ctx.beginPath();
        const gradient = ctx.createLinearGradient(t.x, t.y, t.x + t.size, t.y + t.size);
        gradient.addColorStop(0, "%s");
        gradient.addColorStop(1, "#22c55e");
        ctx.fillStyle = gradient;
        ctx.shadowColor = "rgba(91,141,239,0.55)";
        ctx.shadowBlur = 18;
        ctx.arc(t.x, t.y, t.size / 2, 0, Math.PI * 2);
        ctx.fill();
        ctx.shadowBlur = 0;
      });
      requestAnimationFrame(draw);
    }
    draw();

    function hitTest(x, y) {
      for (let i = 0; i < targets.length; i++) {
        const t = targets[i];
        const dx = x - t.x;
        const dy = y - t.y;
        if (Math.sqrt(dx*dx + dy*dy) <= t.size / 2) {
          targets.splice(i, 1);
          combo = Math.min(combo + 1, 9);
          score += 10 * combo;
          updateUI();
          log("命中！+" + (10 * combo));
          return true;
        }
      }
      combo = 1;
      updateUI();
      log("Miss...");
      return false;
    }

    function updateUI() {
      scoreEl.textContent = score;
      comboEl.textContent = "x" + combo;
    }

    function loop() {
      if (!active) return;
      spawnTarget();
      setTimeout(loop, 700 + Math.random()*400);
    }

    canvas.addEventListener("click", (e) => {
      const rect = canvas.getBoundingClientRect();
      hitTest(e.clientX - rect.left, e.clientY - rect.top);
    });

    window.addEventListener("keydown", (e) => {
      if (e.key === " ") {
        e.preventDefault();
        hitTest(Math.random() * canvas.clientWidth, Math.random() * canvas.clientHeight);
      }
    });

    startBtn.addEventListener("click", () => {
      if (active) {
        active = false;
        startBtn.textContent = "%s";
        log("暂停，成绩可导出。");
        return;
      }
      score = 0;
      combo = 1;
      targets = [];
      active = true;
      startBtn.textContent = "Playing...";
      updateUI();
      log("开始：%s");
      loop();
    });
  </script>
</body>
</html>`, title, theme, title, prompt, aiBlock, cta, theme, cta, prompt)
}

func safeFilename(title string) string {
	safe := strings.Map(func(r rune) rune {
		if r == ' ' || r == '_' || r == '-' {
			return '_'
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, strings.TrimSpace(title))
	if safe == "" {
		return "miniapp"
	}
	return safe
}
