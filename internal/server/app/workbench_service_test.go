package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentports "alex/internal/agent/ports"
	"alex/internal/storage/blobstore"
	"alex/internal/storage/craftsync"
	id "alex/internal/utils/id"
)

type stubAgentExecutor struct {
	session   *agentports.Session
	result    *agentports.TaskResult
	err       error
	taskInput string
}

func (s *stubAgentExecutor) GetSession(ctx context.Context, id string) (*agentports.Session, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.session == nil {
		s.session = &agentports.Session{ID: "session-123"}
	}
	return s.session, nil
}

func (s *stubAgentExecutor) ExecuteTask(ctx context.Context, task string, sessionID string, listener agentports.EventListener) (*agentports.TaskResult, error) {
	s.taskInput = task
	if s.err != nil {
		return nil, s.err
	}
	if s.result == nil {
		return &agentports.TaskResult{Answer: "{}", SessionID: sessionID, TaskID: "task-123"}, nil
	}
	return s.result, nil
}

type stubIllustrationGenerator struct {
	prompts []string
	result  *GeneratedIllustration
	err     error
}

func (s *stubIllustrationGenerator) GenerateIllustration(ctx context.Context, prompt string) (*GeneratedIllustration, error) {
	s.prompts = append(s.prompts, prompt)
	if s.err != nil {
		return nil, s.err
	}
	if s.result == nil {
		return nil, nil
	}
	clone := *s.result
	if len(s.result.Data) > 0 {
		clone.Data = append([]byte(nil), s.result.Data...)
	}
	return &clone, nil
}

type memorySessionStore struct {
	sessions map[string]*agentports.Session
}

func newMemorySessionStore() *memorySessionStore {
	return &memorySessionStore{sessions: make(map[string]*agentports.Session)}
}

func (m *memorySessionStore) Create(ctx context.Context) (*agentports.Session, error) {
	userID := id.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errors.New("missing user context")
	}
	session := &agentports.Session{ID: id.NewSessionID(), UserID: userID, Metadata: map[string]string{}}
	m.sessions[session.ID] = session
	return session, nil
}

func (m *memorySessionStore) Get(ctx context.Context, sessionID string) (*agentports.Session, error) {
	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, errors.New("session not found")
	}
	return session, nil
}

func (m *memorySessionStore) Save(ctx context.Context, session *agentports.Session) error {
	if session == nil {
		return errors.New("nil session")
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *memorySessionStore) List(ctx context.Context) ([]string, error) {
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *memorySessionStore) Delete(ctx context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

type memoryBlobStore struct {
	objects map[string][]byte
}

func newMemoryBlobStore() *memoryBlobStore {
	return &memoryBlobStore{objects: make(map[string][]byte)}
}

func (m *memoryBlobStore) PutObject(ctx context.Context, key string, body io.Reader, opts blobstore.PutOptions) (string, error) {
	if key == "" {
		return "", errors.New("missing key")
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	m.objects[key] = data
	return key, nil
}

func (m *memoryBlobStore) GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return "https://example.com/" + key, nil
}

func (m *memoryBlobStore) DeleteObject(ctx context.Context, key string) error {
	delete(m.objects, key)
	return nil
}

type sessionBackedExecutor struct {
	store *memorySessionStore
}

func (s *sessionBackedExecutor) GetSession(ctx context.Context, id string) (*agentports.Session, error) {
	if id == "" {
		return s.store.Create(ctx)
	}
	return s.store.Get(ctx, id)
}

func (s *sessionBackedExecutor) ExecuteTask(ctx context.Context, task string, sessionID string, listener agentports.EventListener) (*agentports.TaskResult, error) {
	return nil, errors.New("not implemented")
}

func TestGenerateArticleInsightsParsesResponse(t *testing.T) {
	executor := &stubAgentExecutor{
		result: &agentports.TaskResult{
			Answer:    `{"summary":"简要","key_points":["要点一"],"suggestions":["建议"],"citations":[{"title":"报告","source":"机构","url":"https://example.com","snippet":"数据"}],"illustrations":[{"paragraph_summary":"引言段","image_idea":"开场氛围插图","prompt":"intro paragraph, soft light","keywords":["warm light","intro"]}]}`,
			SessionID: "session-321",
			TaskID:    "task-789",
		},
	}

	service := NewWorkbenchService(executor, nil, nil, WithWorkbenchContentLimit(1000))
	ctx := id.WithUserID(context.Background(), "user-1")
	insights, err := service.GenerateArticleInsights(ctx, "<p>测试内容</p>")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if insights.Summary != "简要" {
		t.Fatalf("unexpected summary: %s", insights.Summary)
	}
	if len(insights.KeyPoints) != 1 || insights.KeyPoints[0] != "要点一" {
		t.Fatalf("unexpected key points: %#v", insights.KeyPoints)
	}
	if len(insights.Suggestions) != 1 || insights.Suggestions[0] != "建议" {
		t.Fatalf("unexpected suggestions: %#v", insights.Suggestions)
	}
	if len(insights.Citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(insights.Citations))
	}
	if insights.Citations[0].URL != "https://example.com" {
		t.Fatalf("unexpected citation url: %s", insights.Citations[0].URL)
	}
	if len(insights.Illustrations) != 1 {
		t.Fatalf("expected 1 illustration, got %d", len(insights.Illustrations))
	}
	illustration := insights.Illustrations[0]
	if illustration.ImageIdea != "开场氛围插图" {
		t.Fatalf("unexpected illustration idea: %#v", illustration)
	}
	if illustration.Prompt != "intro paragraph, soft light" {
		t.Fatalf("unexpected illustration prompt: %#v", illustration)
	}
	if len(illustration.Keywords) != 2 || illustration.Keywords[0] != "warm light" {
		t.Fatalf("unexpected illustration keywords: %#v", illustration.Keywords)
	}
	if insights.SessionID != "session-321" || insights.TaskID != "task-789" {
		t.Fatalf("unexpected ids: %#v", insights)
	}
}

func TestGenerateArticleInsightsCreatesIllustrations(t *testing.T) {
	store := newMemorySessionStore()
	blob := newMemoryBlobStore()
	session := &agentports.Session{ID: "session-ill", UserID: "user-ill", Metadata: map[string]string{}}
	store.sessions[session.ID] = session

	executor := &stubAgentExecutor{
		session: session,
		result: &agentports.TaskResult{
			Answer:    `{"summary":"摘要","key_points":[],"suggestions":[],"illustrations":[{"paragraph_summary":"段落","image_idea":"夜晚的城市","prompt":"vibrant city at dusk","keywords":["city"]}]}`,
			SessionID: session.ID,
			TaskID:    "task-ill",
		},
	}

	generator := &stubIllustrationGenerator{
		result: &GeneratedIllustration{
			MediaType: "image/png",
			Data:      []byte{0x01, 0x02, 0x03},
		},
	}

	service := NewWorkbenchService(executor, store, blob, WithIllustrationGenerator(generator))
	ctx := id.WithUserID(context.Background(), "user-ill")

	insights, err := service.GenerateArticleInsights(ctx, "<p>示例内容</p>")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(insights.Illustrations) != 1 {
		t.Fatalf("expected 1 illustration, got %d", len(insights.Illustrations))
	}

	illustration := insights.Illustrations[0]
	if illustration.CraftID == "" {
		t.Fatalf("expected craft id, got empty string")
	}
	if illustration.ImageURL == "" {
		t.Fatalf("expected image url, got empty string")
	}
	if illustration.MediaType != "image/png" {
		t.Fatalf("unexpected media type: %s", illustration.MediaType)
	}
	if illustration.Name == "" {
		t.Fatalf("expected illustration name to be populated")
	}

	sessionAfter, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if len(sessionAfter.Artifacts) != 1 {
		t.Fatalf("expected session to have 1 artifact, got %d", len(sessionAfter.Artifacts))
	}
	artifact := sessionAfter.Artifacts[0]
	if artifact.Source != articleIllustrationSource {
		t.Fatalf("unexpected artifact source: %s", artifact.Source)
	}
	if _, ok := blob.objects[artifact.StorageKey]; !ok {
		t.Fatalf("expected blob to contain key %s", artifact.StorageKey)
	}

	if len(generator.prompts) != 1 || generator.prompts[0] != "vibrant city at dusk" {
		t.Fatalf("unexpected prompts logged: %#v", generator.prompts)
	}
}

func TestGenerateArticleInsightsFallsBackToPlainText(t *testing.T) {
	executor := &stubAgentExecutor{
		result: &agentports.TaskResult{
			Answer:    "无法提供结构化信息，但这里是摘要",
			SessionID: "session-777",
			TaskID:    "task-888",
		},
	}

	service := NewWorkbenchService(executor, nil, nil, WithWorkbenchContentLimit(1000))
	ctx := id.WithUserID(context.Background(), "user-2")
	insights, err := service.GenerateArticleInsights(ctx, "文章内容")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if insights.Summary != "无法提供结构化信息，但这里是摘要" {
		t.Fatalf("expected raw answer fallback, got %s", insights.Summary)
	}
	if insights.SessionID != "session-777" || insights.TaskID != "task-888" {
		t.Fatalf("unexpected ids: %#v", insights)
	}
}

func TestGenerateArticleInsightsValidatesInput(t *testing.T) {
	executor := &stubAgentExecutor{}
	service := NewWorkbenchService(executor, nil, nil)

	if _, err := service.GenerateArticleInsights(context.Background(), "内容"); !errors.Is(err, ErrWorkbenchMissingUser) {
		t.Fatalf("expected ErrWorkbenchMissingUser, got %v", err)
	}

	ctx := id.WithUserID(context.Background(), "user-3")
	if _, err := service.GenerateArticleInsights(ctx, "  "); !errors.Is(err, ErrWorkbenchContentRequired) {
		t.Fatalf("expected ErrWorkbenchContentRequired, got %v", err)
	}

	executor.err = errors.New("boom")
	if _, err := service.GenerateArticleInsights(ctx, "hello"); err == nil {
		t.Fatal("expected executor error")
	}
}

func TestGenerateImageConceptsParsesResponse(t *testing.T) {
	executor := &stubAgentExecutor{
		result: &agentports.TaskResult{
			Answer:    `{"concepts":[{"title":"银河海报","prompt":"cinematic space scene, vibrant nebula","style_notes":["强调冷暖对比"],"aspect_ratio":"16:9","seed_hint":"1234","mood":"史诗"}]}`,
			SessionID: "session-img",
			TaskID:    "task-img",
		},
	}

	service := NewWorkbenchService(executor, nil, nil)
	ctx := id.WithUserID(context.Background(), "user-4")
	result, err := service.GenerateImageConcepts(ctx, GenerateImageConceptsRequest{Brief: "银河主题的宣传海报", Style: "赛博朋克", References: []string{"https://example.com/ref"}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Concepts) != 1 {
		t.Fatalf("expected 1 concept, got %d", len(result.Concepts))
	}
	concept := result.Concepts[0]
	if concept.Title != "银河海报" {
		t.Fatalf("unexpected title: %s", concept.Title)
	}
	if concept.Prompt != "cinematic space scene, vibrant nebula" {
		t.Fatalf("unexpected prompt: %s", concept.Prompt)
	}
	if len(concept.StyleNotes) != 1 || concept.StyleNotes[0] != "强调冷暖对比" {
		t.Fatalf("unexpected style notes: %#v", concept.StyleNotes)
	}
	if concept.AspectRatio != "16:9" {
		t.Fatalf("unexpected aspect ratio: %s", concept.AspectRatio)
	}
	if result.SessionID != "session-img" || result.TaskID != "task-img" {
		t.Fatalf("unexpected metadata: %#v", result)
	}
}

func TestGenerateImageConceptsFallsBackToRawAnswer(t *testing.T) {
	executor := &stubAgentExecutor{
		result: &agentports.TaskResult{Answer: "use neon reflections and rainy city streets", SessionID: "session-x", TaskID: "task-y"},
	}
	service := NewWorkbenchService(executor, nil, nil)
	ctx := id.WithUserID(context.Background(), "user-5")

	result, err := service.GenerateImageConcepts(ctx, GenerateImageConceptsRequest{Brief: "夜晚的未来城市"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Concepts) != 1 {
		t.Fatalf("expected fallback concept, got %d", len(result.Concepts))
	}
	if result.Concepts[0].Prompt != "use neon reflections and rainy city streets" {
		t.Fatalf("unexpected fallback prompt: %s", result.Concepts[0].Prompt)
	}
}

func TestGenerateImageConceptsValidatesInput(t *testing.T) {
	executor := &stubAgentExecutor{}
	service := NewWorkbenchService(executor, nil, nil)

	if _, err := service.GenerateImageConcepts(context.Background(), GenerateImageConceptsRequest{Brief: "灵感"}); !errors.Is(err, ErrWorkbenchMissingUser) {
		t.Fatalf("expected ErrWorkbenchMissingUser, got %v", err)
	}

	ctx := id.WithUserID(context.Background(), "user-6")
	if _, err := service.GenerateImageConcepts(ctx, GenerateImageConceptsRequest{Brief: "   "}); !errors.Is(err, ErrWorkbenchContentRequired) {
		t.Fatalf("expected ErrWorkbenchContentRequired, got %v", err)
	}
}

func TestGenerateWebBlueprintParsesResponse(t *testing.T) {
	executor := &stubAgentExecutor{
		result: &agentports.TaskResult{
			Answer:    `{"blueprint":{"page_title":"未来城 SaaS 落地页","summary":"帮助企业了解未来城产品价值","sections":[{"title":"首屏价值","purpose":"概述产品定位","components":["主标题","副标题","CTA"],"copy_suggestions":["未来城：面向城市运营的实时指挥平台"]},{"title":"功能亮点","purpose":"展示核心模块","components":["图标+文案"],"copy_suggestions":["三大模块覆盖监测、分析、指挥"]}],"call_to_actions":[{"label":"预约演示","destination":"/demo","variant":"primary","messaging":"24 小时内联系"}],"seo_keywords":["城市数字化","SaaS 指挥平台"]}}`,
			SessionID: "session-web",
			TaskID:    "task-web",
		},
	}

	service := NewWorkbenchService(executor, nil, nil)
	ctx := id.WithUserID(context.Background(), "user-web")
	result, err := service.GenerateWebBlueprint(ctx, GenerateWebBlueprintRequest{Goal: "构建未来城 SaaS 产品的营销落地页", Audience: "城市运营管理者", Tone: "专业可信", MustHaves: []string{"展示客户案例"}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Blueprint.PageTitle != "未来城 SaaS 落地页" {
		t.Fatalf("unexpected title: %s", result.Blueprint.PageTitle)
	}
	if len(result.Blueprint.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(result.Blueprint.Sections))
	}
	if result.Blueprint.CallToActions == nil || len(result.Blueprint.CallToActions) != 1 {
		t.Fatalf("expected one CTA, got %#v", result.Blueprint.CallToActions)
	}
	if result.SessionID != "session-web" || result.TaskID != "task-web" {
		t.Fatalf("unexpected metadata: %#v", result)
	}
}

func TestGenerateWebBlueprintFallsBackToTemplate(t *testing.T) {
	executor := &stubAgentExecutor{
		result: &agentports.TaskResult{Answer: "请突出品牌调性"},
	}

	service := NewWorkbenchService(executor, nil, nil)
	ctx := id.WithUserID(context.Background(), "user-template")
	result, err := service.GenerateWebBlueprint(ctx, GenerateWebBlueprintRequest{Goal: "推出新品发布会报名页", MustHaves: []string{"报名表单"}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Blueprint.Sections) < 4 {
		t.Fatalf("expected fallback blueprint with sections, got %d", len(result.Blueprint.Sections))
	}
	if result.Blueprint.CallToActions == nil || len(result.Blueprint.CallToActions) == 0 {
		t.Fatalf("expected fallback CTA")
	}
}

func TestGenerateWebBlueprintValidatesInput(t *testing.T) {
	executor := &stubAgentExecutor{}
	service := NewWorkbenchService(executor, nil, nil)

	if _, err := service.GenerateWebBlueprint(context.Background(), GenerateWebBlueprintRequest{Goal: "落地页"}); !errors.Is(err, ErrWorkbenchMissingUser) {
		t.Fatalf("expected ErrWorkbenchMissingUser, got %v", err)
	}

	ctx := id.WithUserID(context.Background(), "user-empty")
	if _, err := service.GenerateWebBlueprint(ctx, GenerateWebBlueprintRequest{Goal: "   "}); !errors.Is(err, ErrWorkbenchContentRequired) {
		t.Fatalf("expected ErrWorkbenchContentRequired, got %v", err)
	}
}

func TestGenerateCodeServicePlanParsesResponse(t *testing.T) {
	executor := &stubAgentExecutor{
		result: &agentports.TaskResult{
			Answer:    `{"plan":{"service_name":"Invoice Hub","summary":"Generate compliant invoices","language":"Go","runtime":"Go + chi","architecture":["Hexagonal architecture"],"components":[{"name":"API Layer","responsibility":"Handle HTTP requests","tech_notes":["Use chi router"]},{"name":"InvoiceService","responsibility":"Generate invoices","tech_notes":["Render PDF"]}],"api_endpoints":[{"method":"post","path":"/api/invoices","description":"Create invoice","request_schema":"{\"customer_id\":string}","response_schema":"{\"id\":string}"}],"dev_tasks":["Write integration tests"],"operations":["Expose Prometheus metrics"],"testing":["Run go test ./..."]}}`,
			SessionID: "session-code",
			TaskID:    "task-code",
		},
	}

	service := NewWorkbenchService(executor, nil, nil)
	ctx := id.WithUserID(context.Background(), "user-code")
	result, err := service.GenerateCodeServicePlan(ctx, GenerateCodeServicePlanRequest{
		ServiceName:  " Invoice Hub ",
		Objective:    " 构建自动化发票生成与归档 API ",
		Language:     "Go",
		Features:     []string{"生成发票 PDF", "同步 CRM"},
		Integrations: []string{"PostgreSQL", "Stripe"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Plan.Summary != "Generate compliant invoices" {
		t.Fatalf("unexpected summary: %s", result.Plan.Summary)
	}
	if result.Plan.Language != "Go" {
		t.Fatalf("unexpected language: %s", result.Plan.Language)
	}
	if result.Plan.Runtime != "Go + chi" {
		t.Fatalf("unexpected runtime: %s", result.Plan.Runtime)
	}
	if len(result.Plan.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(result.Plan.Components))
	}
	if len(result.Plan.APIEndpoints) == 0 || result.Plan.APIEndpoints[0].Method != "POST" {
		t.Fatalf("expected parsed endpoint with POST method, got %#v", result.Plan.APIEndpoints)
	}
	if result.SessionID != "session-code" || result.TaskID != "task-code" {
		t.Fatalf("unexpected metadata: %+v", result)
	}
	if !strings.Contains(executor.taskInput, "Invoice Hub") {
		t.Fatalf("expected prompt to include sanitized service name, got %q", executor.taskInput)
	}
}

func TestGenerateCodeServicePlanFallsBack(t *testing.T) {
	executor := &stubAgentExecutor{
		result: &agentports.TaskResult{
			Answer:    "not-json",
			SessionID: "session-fallback",
			TaskID:    "task-fallback",
		},
	}

	service := NewWorkbenchService(executor, nil, nil)
	ctx := id.WithUserID(context.Background(), "user-fallback")
	result, err := service.GenerateCodeServicePlan(ctx, GenerateCodeServicePlanRequest{
		ServiceName:  "Sandbox Service",
		Objective:    "提供演示 API 并写入 crafts",
		Language:     "python",
		Features:     []string{"同步日志", "Webhook 回调"},
		Integrations: []string{"Redis"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Plan.APIEndpoints) < 2 {
		t.Fatalf("expected fallback endpoints, got %#v", result.Plan.APIEndpoints)
	}
	if !strings.EqualFold(result.Plan.Language, "Python") {
		t.Fatalf("expected language fallback to Python, got %s", result.Plan.Language)
	}
	if result.Plan.Runtime == "" {
		t.Fatalf("expected runtime fallback")
	}
	if len(result.Plan.DevTasks) == 0 {
		t.Fatalf("expected fallback dev tasks")
	}
}

func TestGenerateCodeServicePlanValidatesInput(t *testing.T) {
	service := NewWorkbenchService(&stubAgentExecutor{}, nil, nil)
	ctx := id.WithUserID(context.Background(), "user-validate")

	if _, err := service.GenerateCodeServicePlan(context.Background(), GenerateCodeServicePlanRequest{ServiceName: "svc", Objective: "demo"}); !errors.Is(err, ErrWorkbenchMissingUser) {
		t.Fatalf("expected ErrWorkbenchMissingUser, got %v", err)
	}

	if _, err := service.GenerateCodeServicePlan(ctx, GenerateCodeServicePlanRequest{ServiceName: "  ", Objective: "demo"}); !errors.Is(err, ErrWorkbenchServiceNameRequired) {
		t.Fatalf("expected ErrWorkbenchServiceNameRequired, got %v", err)
	}

	if _, err := service.GenerateCodeServicePlan(ctx, GenerateCodeServicePlanRequest{ServiceName: "svc", Objective: "   "}); !errors.Is(err, ErrWorkbenchContentRequired) {
		t.Fatalf("expected ErrWorkbenchContentRequired, got %v", err)
	}
}

func TestParseWebBlueprintHandlesOptionalFields(t *testing.T) {
	answer := `{"blueprint":{"page_title":"科技发布会落地页","summary":"帮助用户报名最新发布会","sections":[{"title":"首屏","purpose":"传达主旨","components":[" 主标题 ",""],"copy_suggestions":[" 立即报名 ",""]}],"call_to_actions":[{"label":"马上报名","destination":" /signup ","variant":"primary","messaging":"限额 200 人"},{"label":"缺少链接","destination":""}],"seo_keywords":[" 发布会 ",""]}}`

	blueprint, err := parseWebBlueprint(answer)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(blueprint.Sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(blueprint.Sections))
	}
	if len(blueprint.Sections[0].Components) != 1 || blueprint.Sections[0].Components[0] != "主标题" {
		t.Fatalf("expected normalized components, got %#v", blueprint.Sections[0].Components)
	}
	if len(blueprint.Sections[0].CopySuggestions) != 1 || blueprint.Sections[0].CopySuggestions[0] != "立即报名" {
		t.Fatalf("expected normalized copy suggestions, got %#v", blueprint.Sections[0].CopySuggestions)
	}
	if len(blueprint.CallToActions) != 1 || blueprint.CallToActions[0].Destination != "/signup" {
		t.Fatalf("expected filtered CTA destination, got %#v", blueprint.CallToActions)
	}
	if len(blueprint.SEOKeywords) != 1 || blueprint.SEOKeywords[0] != "发布会" {
		t.Fatalf("expected trimmed SEO keywords, got %#v", blueprint.SEOKeywords)
	}
}

func TestParseWebBlueprintRejectsInvalidPayloads(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"plain text",
		`{"blueprint":{"page_title":"","summary":"","sections":[]}}`,
		`{"blueprint":{"page_title":"缺少段落","summary":"只有标题","sections":[{"title":"","purpose":""}]}}`,
	}

	for _, input := range cases {
		input := input
		t.Run(fmt.Sprintf("case_%x", sha256.Sum256([]byte(input))), func(t *testing.T) {
			t.Parallel()
			if _, err := parseWebBlueprint(input); err == nil {
				t.Fatalf("expected error for input %q", input)
			}
		})
	}
}

func TestSaveArticleDraftCreatesCraft(t *testing.T) {
	store := newMemorySessionStore()
	blob := newMemoryBlobStore()
	executor := &sessionBackedExecutor{store: store}
	service := NewWorkbenchService(executor, store, blob)

	ctx := id.WithUserID(context.Background(), "user-save")
	result, err := service.SaveArticleDraft(ctx, SaveArticleDraftRequest{
		Title:   "测试标题",
		Content: "<h1>测试标题</h1><p>段落内容</p>",
		Summary: "简要总结",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.SessionID == "" {
		t.Fatal("expected session id in response")
	}
	if result.Craft.ID == "" || result.Craft.StorageKey == "" {
		t.Fatalf("expected craft metadata, got %#v", result.Craft)
	}
	if !strings.HasSuffix(result.Craft.Name, ".html") {
		t.Fatalf("expected .html suffix, got %s", result.Craft.Name)
	}
	if _, ok := blob.objects[result.Craft.StorageKey]; !ok {
		t.Fatalf("expected blob to be stored with key %s", result.Craft.StorageKey)
	}

	session, err := store.Get(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if len(session.Artifacts) != 1 {
		t.Fatalf("expected session to have 1 artifact, got %d", len(session.Artifacts))
	}
}

func TestSaveArticleDraftMirrorsToFilesystem(t *testing.T) {
	store := newMemorySessionStore()
	blob := newMemoryBlobStore()
	executor := &sessionBackedExecutor{store: store}
	mirrorDir := t.TempDir()
	mirror, err := craftsync.NewFilesystemMirror(mirrorDir)
	if err != nil {
		t.Fatalf("NewFilesystemMirror returned error: %v", err)
	}
	service := NewWorkbenchService(executor, store, blob, WithCraftMirror(mirror))

	ctx := id.WithUserID(context.Background(), "mirror-user")
	result, err := service.SaveArticleDraft(ctx, SaveArticleDraftRequest{Title: "镜像文章", Content: "<p>Hello</p>"})
	if err != nil {
		t.Fatalf("SaveArticleDraft returned error: %v", err)
	}

	pattern := filepath.Join(mirrorDir, "*", "*", result.Craft.ID, "metadata.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one metadata file, got %d", len(matches))
	}

	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("failed to read metadata: %v", err)
	}

	var payload struct {
		Name          string `json:"name"`
		MediaType     string `json:"media_type"`
		LocalFilename string `json:"local_filename"`
		UserID        string `json:"user_id"`
		SessionID     string `json:"session_id"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to parse metadata: %v", err)
	}
	if payload.LocalFilename == "" {
		t.Fatalf("expected local filename to be recorded")
	}
	if payload.MediaType != "text/html" {
		t.Fatalf("unexpected media type: %s", payload.MediaType)
	}
	if payload.UserID != "mirror-user" {
		t.Fatalf("expected user id mirror-user, got %s", payload.UserID)
	}
	if payload.SessionID == "" {
		t.Fatalf("expected session id recorded, got empty string")
	}

	contentPath := filepath.Join(filepath.Dir(matches[0]), payload.LocalFilename)
	if _, err := os.Stat(contentPath); err != nil {
		t.Fatalf("expected mirrored content file, got error: %v", err)
	}
}

func TestDeleteArticleDraftRemovesMirror(t *testing.T) {
	store := newMemorySessionStore()
	blob := newMemoryBlobStore()
	executor := &sessionBackedExecutor{store: store}
	mirrorDir := t.TempDir()
	mirror, err := craftsync.NewFilesystemMirror(mirrorDir)
	if err != nil {
		t.Fatalf("NewFilesystemMirror returned error: %v", err)
	}
	service := NewWorkbenchService(executor, store, blob, WithCraftMirror(mirror))

	ctx := id.WithUserID(context.Background(), "mirror-user")
	result, err := service.SaveArticleDraft(ctx, SaveArticleDraftRequest{Title: "要删除的草稿", Content: "<p>删除</p>"})
	if err != nil {
		t.Fatalf("SaveArticleDraft returned error: %v", err)
	}

	pattern := filepath.Join(mirrorDir, "*", "*", result.Craft.ID, "metadata.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one metadata file before deletion, got %d", len(matches))
	}

	if err := service.DeleteArticleDraft(ctx, result.Craft.ID); err != nil {
		t.Fatalf("DeleteArticleDraft returned error: %v", err)
	}

	matches, err = filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob after deletion failed: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected metadata file to be removed, found %d", len(matches))
	}
}

func TestSaveArticleDraftRequiresUser(t *testing.T) {
	store := newMemorySessionStore()
	blob := newMemoryBlobStore()
	executor := &sessionBackedExecutor{store: store}
	service := NewWorkbenchService(executor, store, blob)

	if _, err := service.SaveArticleDraft(context.Background(), SaveArticleDraftRequest{Content: "hello"}); !errors.Is(err, ErrWorkbenchMissingUser) {
		t.Fatalf("expected missing user error, got %v", err)
	}
}

func TestSaveArticleDraftReusesSession(t *testing.T) {
	store := newMemorySessionStore()
	blob := newMemoryBlobStore()
	executor := &sessionBackedExecutor{store: store}
	service := NewWorkbenchService(executor, store, blob)

	ctx := id.WithUserID(context.Background(), "user-reuse")
	first, err := service.SaveArticleDraft(ctx, SaveArticleDraftRequest{Content: "<p>一次</p>"})
	if err != nil {
		t.Fatalf("first save failed: %v", err)
	}
	second, err := service.SaveArticleDraft(ctx, SaveArticleDraftRequest{SessionID: first.SessionID, Content: "<p>两次</p>"})
	if err != nil {
		t.Fatalf("second save failed: %v", err)
	}
	if second.SessionID != first.SessionID {
		t.Fatalf("expected same session, got %s vs %s", second.SessionID, first.SessionID)
	}
	session, err := store.Get(ctx, first.SessionID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if len(session.Artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(session.Artifacts))
	}
}

func TestListArticleDraftsReturnsSignedURLs(t *testing.T) {
	store := newMemorySessionStore()
	blob := newMemoryBlobStore()
	executor := &sessionBackedExecutor{store: store}
	service := NewWorkbenchService(executor, store, blob)

	ctx := id.WithUserID(context.Background(), "user-1")

	result1, err := service.SaveArticleDraft(ctx, SaveArticleDraftRequest{Title: "第一次草稿", Content: "<p>hello world</p>"})
	if err != nil {
		t.Fatalf("failed to save draft: %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	result2, err := service.SaveArticleDraft(ctx, SaveArticleDraftRequest{Title: "第二次草稿", Content: "<p>another</p>"})
	if err != nil {
		t.Fatalf("failed to save second draft: %v", err)
	}

	session, err := store.Get(ctx, result1.SessionID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	session.Artifacts = append(session.Artifacts, agentports.Artifact{ID: "ignored", Source: "other", CreatedAt: time.Now()})
	if err := store.Save(ctx, session); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	drafts, err := service.ListArticleDrafts(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(drafts) != 2 {
		t.Fatalf("expected 2 drafts, got %d", len(drafts))
	}

	if drafts[0].Craft.ID != result2.Craft.ID {
		t.Fatalf("expected most recent draft first, got %s", drafts[0].Craft.ID)
	}
	if drafts[0].DownloadURL == "" || drafts[1].DownloadURL == "" {
		t.Fatalf("expected download urls, got %#v", drafts)
	}
}

func TestListArticleDraftsRequiresUser(t *testing.T) {
	store := newMemorySessionStore()
	blob := newMemoryBlobStore()
	executor := &sessionBackedExecutor{store: store}
	service := NewWorkbenchService(executor, store, blob)

	if _, err := service.ListArticleDrafts(context.Background()); !errors.Is(err, ErrWorkbenchMissingUser) {
		t.Fatalf("expected ErrWorkbenchMissingUser, got %v", err)
	}
}

func TestDeleteArticleDraftRemovesArtifactAndBlob(t *testing.T) {
	store := newMemorySessionStore()
	blob := newMemoryBlobStore()
	executor := &sessionBackedExecutor{store: store}
	service := NewWorkbenchService(executor, store, blob)

	ctx := id.WithUserID(context.Background(), "user-delete")
	result, err := service.SaveArticleDraft(ctx, SaveArticleDraftRequest{Content: "<p>hello world</p>"})
	if err != nil {
		t.Fatalf("failed to save draft: %v", err)
	}

	sessionBefore, err := store.Get(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("failed to load session: %v", err)
	}
	if len(sessionBefore.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact before deletion, got %d", len(sessionBefore.Artifacts))
	}

	if err := service.DeleteArticleDraft(ctx, result.Craft.ID); err != nil {
		t.Fatalf("expected delete success, got %v", err)
	}

	sessionAfter, err := store.Get(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if len(sessionAfter.Artifacts) != 0 {
		t.Fatalf("expected artifacts removed, got %d", len(sessionAfter.Artifacts))
	}

	if _, ok := blob.objects[result.Craft.StorageKey]; ok {
		t.Fatalf("expected blob %s to be deleted", result.Craft.StorageKey)
	}
}

func TestDeleteArticleDraftNotFound(t *testing.T) {
	store := newMemorySessionStore()
	blob := newMemoryBlobStore()
	executor := &sessionBackedExecutor{store: store}
	service := NewWorkbenchService(executor, store, blob)

	ctx := id.WithUserID(context.Background(), "user-missing")
	if err := service.DeleteArticleDraft(ctx, "missing-id"); !errors.Is(err, ErrWorkbenchDraftNotFound) {
		t.Fatalf("expected ErrWorkbenchDraftNotFound, got %v", err)
	}
}

func TestDeleteArticleDraftRequiresUser(t *testing.T) {
	store := newMemorySessionStore()
	blob := newMemoryBlobStore()
	executor := &sessionBackedExecutor{store: store}
	service := NewWorkbenchService(executor, store, blob)

	if err := service.DeleteArticleDraft(context.Background(), "craft-id"); !errors.Is(err, ErrWorkbenchMissingUser) {
		t.Fatalf("expected ErrWorkbenchMissingUser, got %v", err)
	}

	ctx := id.WithUserID(context.Background(), "user-empty")
	if err := service.DeleteArticleDraft(ctx, "   "); err == nil {
		t.Fatal("expected error for empty craft id")
	}
}
