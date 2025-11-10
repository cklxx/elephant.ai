package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	agentPorts "alex/internal/agent/ports"
	"alex/internal/server/app"
	"alex/internal/storage/blobstore"
	id "alex/internal/utils/id"
)

type failingAgentCoordinator struct {
	err error
}

func (f *failingAgentCoordinator) GetSession(ctx context.Context, id string) (*agentPorts.Session, error) {
	return nil, f.err
}

func (f *failingAgentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agentPorts.EventListener) (*agentPorts.TaskResult, error) {
	return nil, errors.New("not implemented")
}

type workbenchStubExecutor struct {
	answer string
	err    error
}

func (w *workbenchStubExecutor) GetSession(ctx context.Context, id string) (*agentPorts.Session, error) {
	if w.err != nil {
		return nil, w.err
	}
	return &agentPorts.Session{ID: "session-test"}, nil
}

func (w *workbenchStubExecutor) ExecuteTask(ctx context.Context, task string, sessionID string, listener agentPorts.EventListener) (*agentPorts.TaskResult, error) {
	if w.err != nil {
		return nil, w.err
	}
	return &agentPorts.TaskResult{Answer: w.answer, SessionID: sessionID, TaskID: "task-test"}, nil
}

type handlerSessionStore struct {
	sessions map[string]*agentPorts.Session
}

func newHandlerSessionStore() *handlerSessionStore {
	return &handlerSessionStore{sessions: make(map[string]*agentPorts.Session)}
}

func (s *handlerSessionStore) Create(ctx context.Context) (*agentPorts.Session, error) {
	userID := id.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errors.New("missing user")
	}
	session := &agentPorts.Session{ID: id.NewSessionID(), UserID: userID, Metadata: map[string]string{}}
	s.sessions[session.ID] = session
	return session, nil
}

func (s *handlerSessionStore) Get(ctx context.Context, sessionID string) (*agentPorts.Session, error) {
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, errors.New("not found")
	}
	return session, nil
}

func (s *handlerSessionStore) Save(ctx context.Context, session *agentPorts.Session) error {
	s.sessions[session.ID] = session
	return nil
}

func (s *handlerSessionStore) List(ctx context.Context) ([]string, error) {
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *handlerSessionStore) Delete(ctx context.Context, id string) error {
	delete(s.sessions, id)
	return nil
}

type handlerBlobStore struct {
	objects map[string][]byte
}

func newHandlerBlobStore() *handlerBlobStore {
	return &handlerBlobStore{objects: make(map[string][]byte)}
}

func (b *handlerBlobStore) PutObject(ctx context.Context, key string, body io.Reader, opts blobstore.PutOptions) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	b.objects[key] = data
	return key, nil
}

func (b *handlerBlobStore) GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return "https://example.com/" + key, nil
}

func (b *handlerBlobStore) DeleteObject(ctx context.Context, key string) error {
	delete(b.objects, key)
	return nil
}

type handlerExecutor struct {
	store *handlerSessionStore
}

func (e *handlerExecutor) GetSession(ctx context.Context, id string) (*agentPorts.Session, error) {
	if id == "" {
		return e.store.Create(ctx)
	}
	return e.store.Get(ctx, id)
}

func (e *handlerExecutor) ExecuteTask(ctx context.Context, task string, sessionID string, listener agentPorts.EventListener) (*agentPorts.TaskResult, error) {
	return nil, errors.New("not implemented")
}

func TestHandleCreateTaskReturnsJSONErrorOnSessionDecodeFailure(t *testing.T) {
	rootErr := errors.New("json: cannot unmarshal object into Go struct field ToolResult.messages.tool_results.error of type error")
	coordinator := app.NewServerCoordinator(&failingAgentCoordinator{err: rootErr}, app.NewEventBroadcaster(), nil, nil)
	handler := NewAPIHandler(coordinator, app.NewHealthChecker(), nil, nil)

	reqBody := bytes.NewBufferString(`{"task":"demo"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", reqBody)
	req = req.WithContext(id.WithUserID(req.Context(), "test-user"))
	rr := httptest.NewRecorder()

	handler.HandleCreateTask(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("expected JSON content type, got %s", contentType)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if resp["error"] != "Failed to create task" {
		t.Fatalf("expected error message 'Failed to create task', got %s", resp["error"])
	}

	expectedDetails := "failed to get/create session: " + rootErr.Error()
	if resp["details"] != expectedDetails {
		t.Fatalf("expected details %q, got %q", expectedDetails, resp["details"])
	}
}

func TestHandleGenerateArticleInsightsSuccess(t *testing.T) {
	executor := &workbenchStubExecutor{answer: `{"summary":"概括","key_points":["要点"],"suggestions":[],"citations":[]}`}
	service := app.NewWorkbenchService(executor, nil, nil)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	req := httptest.NewRequest(http.MethodPost, "/api/workbench/article/insights", bytes.NewBufferString(`{"content":"<p>hello</p>"}`))
	req = req.WithContext(id.WithUserID(req.Context(), "user-123"))
	rr := httptest.NewRecorder()

	handler.HandleGenerateArticleInsights(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp app.ArticleInsights
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Summary != "概括" {
		t.Fatalf("unexpected summary: %s", resp.Summary)
	}
	if resp.SessionID == "" || resp.TaskID == "" {
		t.Fatalf("expected session/task ids in response: %#v", resp)
	}
}

func TestHandleGenerateArticleInsightsRequiresAuth(t *testing.T) {
	executor := &workbenchStubExecutor{answer: `{"summary":"概括","key_points":[],"suggestions":[],"citations":[]}`}
	service := app.NewWorkbenchService(executor, nil, nil)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	req := httptest.NewRequest(http.MethodPost, "/api/workbench/article/insights", bytes.NewBufferString(`{"content":"hi"}`))
	rr := httptest.NewRecorder()

	handler.HandleGenerateArticleInsights(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestHandleGenerateArticleInsightsValidatesContent(t *testing.T) {
	executor := &workbenchStubExecutor{answer: `{"summary":"概括","key_points":[],"suggestions":[],"citations":[]}`}
	service := app.NewWorkbenchService(executor, nil, nil)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	req := httptest.NewRequest(http.MethodPost, "/api/workbench/article/insights", bytes.NewBufferString(`{"content":"   "}`))
	req = req.WithContext(id.WithUserID(req.Context(), "user-321"))
	rr := httptest.NewRecorder()

	handler.HandleGenerateArticleInsights(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleGenerateImageConceptsSuccess(t *testing.T) {
	executor := &workbenchStubExecutor{answer: `{"concepts":[{"title":"概念","prompt":"cinematic city","style_notes":["霓虹光"],"aspect_ratio":"16:9"}]}`}
	service := app.NewWorkbenchService(executor, nil, nil)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	payload := `{"brief":"未来城市夜景","style":"赛博朋克","references":["https://example.com"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/workbench/image/concepts", bytes.NewBufferString(payload))
	req = req.WithContext(id.WithUserID(req.Context(), "user-1"))
	rr := httptest.NewRecorder()

	handler.HandleGenerateImageConcepts(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp app.ImageConceptResult
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Concepts) != 1 || resp.Concepts[0].Prompt != "cinematic city" {
		t.Fatalf("unexpected concepts: %#v", resp.Concepts)
	}
}

func TestHandleGenerateImageConceptsValidatesBrief(t *testing.T) {
	executor := &workbenchStubExecutor{answer: `{"concepts":[]}`}
	service := app.NewWorkbenchService(executor, nil, nil)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	req := httptest.NewRequest(http.MethodPost, "/api/workbench/image/concepts", bytes.NewBufferString(`{"brief":"   "}`))
	req = req.WithContext(id.WithUserID(req.Context(), "user-2"))
	rr := httptest.NewRecorder()

	handler.HandleGenerateImageConcepts(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleGenerateImageConceptsRequiresAuth(t *testing.T) {
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, app.NewWorkbenchService(nil, nil, nil))
	req := httptest.NewRequest(http.MethodPost, "/api/workbench/image/concepts", bytes.NewBufferString(`{"brief":"天空"}`))
	rr := httptest.NewRecorder()

	handler.HandleGenerateImageConcepts(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestHandleGenerateWebBlueprintSuccess(t *testing.T) {
	executor := &workbenchStubExecutor{answer: `{"blueprint":{"page_title":"Alex 登陆页","summary":"吸引创作者注册","sections":[{"title":"首屏价值","purpose":"说明平台优势","components":["标题","副标题"],"copy_suggestions":["与 AI 协同的创作工作台"]}],"call_to_actions":[{"label":"立即体验","destination":"/signup","variant":"primary","messaging":"3 分钟完成注册"}],"seo_keywords":["创作平台","AI 工作流"]}}`}
	service := app.NewWorkbenchService(executor, nil, nil)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	payload := `{"goal":"设计 Alex 平台的注册落地页","audience":"内容创作者","tone":"专业可信","must_haves":["展示核心功能"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/workbench/web/blueprint", bytes.NewBufferString(payload))
	req = req.WithContext(id.WithUserID(req.Context(), "user-web"))
	rr := httptest.NewRecorder()

	handler.HandleGenerateWebBlueprint(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp webBlueprintResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Blueprint.PageTitle != "Alex 登陆页" {
		t.Fatalf("unexpected blueprint: %#v", resp.Blueprint)
	}
	if len(resp.Blueprint.Sections) == 0 {
		t.Fatalf("expected sections in blueprint")
	}
}

func TestHandleGenerateWebBlueprintValidatesGoal(t *testing.T) {
	executor := &workbenchStubExecutor{answer: `{"blueprint":{}}`}
	service := app.NewWorkbenchService(executor, nil, nil)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	req := httptest.NewRequest(http.MethodPost, "/api/workbench/web/blueprint", bytes.NewBufferString(`{"goal":"   "}`))
	req = req.WithContext(id.WithUserID(req.Context(), "user-web"))
	rr := httptest.NewRecorder()

	handler.HandleGenerateWebBlueprint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleGenerateWebBlueprintRequiresAuth(t *testing.T) {
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, app.NewWorkbenchService(nil, nil, nil))
	req := httptest.NewRequest(http.MethodPost, "/api/workbench/web/blueprint", bytes.NewBufferString(`{"goal":"设计官网"}`))
	rr := httptest.NewRecorder()

	handler.HandleGenerateWebBlueprint(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestHandleSaveArticleDraftSuccess(t *testing.T) {
	store := newHandlerSessionStore()
	blob := newHandlerBlobStore()
	executor := &handlerExecutor{store: store}
	service := app.NewWorkbenchService(executor, store, blob)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	payload := `{"title":"示例标题","content":"<p>内容</p>","summary":"摘要"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workbench/article/crafts", bytes.NewBufferString(payload))
	req = req.WithContext(id.WithUserID(req.Context(), "user-abc"))
	rr := httptest.NewRecorder()

	handler.HandleSaveArticleDraft(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp app.SaveArticleDraftResult
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.SessionID == "" || resp.Craft.ID == "" {
		t.Fatalf("expected craft result, got %#v", resp)
	}
}

func TestHandleListArticleDraftsSuccess(t *testing.T) {
	store := newHandlerSessionStore()
	blob := newHandlerBlobStore()
	executor := &handlerExecutor{store: store}
	service := app.NewWorkbenchService(executor, store, blob)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	ctx := id.WithUserID(context.Background(), "user-drafts")
	if _, err := service.SaveArticleDraft(ctx, app.SaveArticleDraftRequest{Title: "草稿一", Content: "<p>first</p>"}); err != nil {
		t.Fatalf("failed to seed draft: %v", err)
	}
	if _, err := service.SaveArticleDraft(ctx, app.SaveArticleDraftRequest{Title: "草稿二", Content: "<p>second</p>"}); err != nil {
		t.Fatalf("failed to seed second draft: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/workbench/article/crafts", nil)
	req = req.WithContext(id.WithUserID(req.Context(), "user-drafts"))
	rr := httptest.NewRecorder()

	handler.HandleListArticleDrafts(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp articleDraftListResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Drafts) < 2 {
		t.Fatalf("expected at least 2 drafts, got %d", len(resp.Drafts))
	}
	if resp.Drafts[0].DownloadURL == "" {
		t.Fatalf("expected download url in response: %#v", resp)
	}
}

func TestHandleListArticleDraftsRequiresAuth(t *testing.T) {
	store := newHandlerSessionStore()
	blob := newHandlerBlobStore()
	executor := &handlerExecutor{store: store}
	service := app.NewWorkbenchService(executor, store, blob)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	req := httptest.NewRequest(http.MethodGet, "/api/workbench/article/crafts", nil)
	rr := httptest.NewRecorder()

	handler.HandleListArticleDrafts(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestHandleDeleteArticleDraftSuccess(t *testing.T) {
	store := newHandlerSessionStore()
	blob := newHandlerBlobStore()
	executor := &handlerExecutor{store: store}
	service := app.NewWorkbenchService(executor, store, blob)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	ctx := id.WithUserID(context.Background(), "user-delete")
	result, err := service.SaveArticleDraft(ctx, app.SaveArticleDraftRequest{Content: "<p>删除测试</p>"})
	if err != nil {
		t.Fatalf("failed to seed draft: %v", err)
	}

	path := "/api/workbench/article/crafts/" + result.Craft.ID
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req = req.WithContext(id.WithUserID(req.Context(), "user-delete"))
	rr := httptest.NewRecorder()

	handler.HandleDeleteArticleDraft(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rr.Code)
	}

	session, err := store.Get(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if len(session.Artifacts) != 0 {
		t.Fatalf("expected artifacts removed, got %d", len(session.Artifacts))
	}
	if _, ok := blob.objects[result.Craft.StorageKey]; ok {
		t.Fatalf("expected blob %s removed", result.Craft.StorageKey)
	}
}

func TestHandleDeleteArticleDraftNotFound(t *testing.T) {
	store := newHandlerSessionStore()
	blob := newHandlerBlobStore()
	executor := &handlerExecutor{store: store}
	service := app.NewWorkbenchService(executor, store, blob)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	req := httptest.NewRequest(http.MethodDelete, "/api/workbench/article/crafts/unknown", nil)
	req = req.WithContext(id.WithUserID(req.Context(), "user-delete"))
	rr := httptest.NewRecorder()

	handler.HandleDeleteArticleDraft(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestHandleDeleteArticleDraftRequiresAuth(t *testing.T) {
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, app.NewWorkbenchService(nil, nil, nil))

	req := httptest.NewRequest(http.MethodDelete, "/api/workbench/article/crafts/anything", nil)
	rr := httptest.NewRecorder()

	handler.HandleDeleteArticleDraft(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestHandleGenerateCodePlanSuccess(t *testing.T) {
	executor := &workbenchStubExecutor{answer: `{"plan":{"service_name":"Demo","summary":"Build a demo","language":"Python","runtime":"Python + FastAPI","architecture":["Layered"],"components":[{"name":"API","responsibility":"Serve HTTP","tech_notes":["Use FastAPI"]}],"api_endpoints":[{"method":"post","path":"/api/demo","description":"Create demo"}],"dev_tasks":["Write tests"],"operations":["Expose metrics"],"testing":["Run pytest"]}}`}
	service := app.NewWorkbenchService(executor, nil, nil)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	req := httptest.NewRequest(http.MethodPost, "/api/workbench/code/plan", bytes.NewBufferString(`{"service_name":"Demo","objective":"构建新的微服务","language":"python","features":["Webhook"],"integrations":["Redis"]}`))
	req = req.WithContext(id.WithUserID(req.Context(), "user-code"))
	rr := httptest.NewRecorder()

	handler.HandleGenerateCodePlan(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp codePlanResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Plan.ServiceName != "Demo" {
		t.Fatalf("unexpected plan: %#v", resp.Plan)
	}
	if len(resp.Plan.Components) != 1 {
		t.Fatalf("expected parsed components, got %#v", resp.Plan.Components)
	}
}

func TestHandleGenerateCodePlanValidatesInput(t *testing.T) {
	service := app.NewWorkbenchService(&workbenchStubExecutor{answer: `{"plan":{}}`}, nil, nil)
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, service)

	req := httptest.NewRequest(http.MethodPost, "/api/workbench/code/plan", bytes.NewBufferString(`{"service_name":"","objective":"demo"}`))
	req = req.WithContext(id.WithUserID(req.Context(), "user-code"))
	rr := httptest.NewRecorder()

	handler.HandleGenerateCodePlan(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/workbench/code/plan", bytes.NewBufferString(`{"service_name":"Demo","objective":"   "}`))
	req = req.WithContext(id.WithUserID(req.Context(), "user-code"))
	rr = httptest.NewRecorder()

	handler.HandleGenerateCodePlan(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleGenerateCodePlanRequiresAuth(t *testing.T) {
	handler := NewAPIHandler(nil, app.NewHealthChecker(), nil, app.NewWorkbenchService(nil, nil, nil))
	req := httptest.NewRequest(http.MethodPost, "/api/workbench/code/plan", bytes.NewBufferString(`{"service_name":"demo","objective":"构建"}`))
	rr := httptest.NewRecorder()

	handler.HandleGenerateCodePlan(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}
