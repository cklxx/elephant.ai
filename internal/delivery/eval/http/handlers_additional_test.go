package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"alex/evaluation/rl"
	"alex/evaluation/task_mgmt"
	serverApp "alex/internal/delivery/server/app"
)

func newEvalHTTPTestDeps(t *testing.T) (*evalHandler, *rlHandler, *taskMgmtHandler) {
	t.Helper()

	evalSvc, err := serverApp.NewEvaluationService(t.TempDir())
	if err != nil {
		t.Fatalf("NewEvaluationService() error = %v", err)
	}
	rlStorage, err := rl.NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorage() error = %v", err)
	}
	taskStore, err := task_mgmt.NewTaskStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTaskStore() error = %v", err)
	}

	cfg := rl.DefaultQualityConfig()
	return &evalHandler{evaluation: evalSvc}, &rlHandler{
			storage:     rlStorage,
			qualityGate: rl.NewQualityGate(cfg, nil),
			config:      cfg,
		}, &taskMgmtHandler{
			manager: task_mgmt.NewTaskManager(taskStore),
		}
}

func TestRLHandlerReadEndpoints(t *testing.T) {
	_, rlH, _ := newEvalHTTPTestDeps(t)

	trajs := []*rl.RLTrajectory{
		{
			ID:          "gold-1",
			QualityTier: rl.TierGold,
			ExtractedAt: time.Date(2026, time.March, 11, 10, 0, 0, 0, time.UTC),
		},
		{
			ID:          "silver-1",
			QualityTier: rl.TierSilver,
			ExtractedAt: time.Date(2026, time.March, 12, 10, 0, 0, 0, time.UTC),
		},
	}
	for _, traj := range trajs {
		if err := rlH.storage.Append(traj); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	t.Run("stats returns manifest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/rl/stats", nil)
		rec := httptest.NewRecorder()

		rlH.handleGetStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var manifest rl.Manifest
		if err := json.Unmarshal(rec.Body.Bytes(), &manifest); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if manifest.Tiers[rl.TierGold].TotalCount != 1 {
			t.Fatalf("gold count = %d, want 1", manifest.Tiers[rl.TierGold].TotalCount)
		}
	})

	t.Run("list trajectories defaults to gold tier and respects date filters", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodGet,
			"/api/rl/trajectories?after=2026-03-11&before=2026-03-11",
			nil,
		)
		rec := httptest.NewRecorder()

		rlH.handleListTrajectories(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var payload struct {
			Tier         rl.QualityTier     `json:"tier"`
			Count        int                `json:"count"`
			Trajectories []*rl.RLTrajectory `json:"trajectories"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if payload.Tier != rl.TierGold || payload.Count != 1 {
			t.Fatalf("payload = %#v, want default gold tier with one result", payload)
		}
	})

	t.Run("get trajectory handles missing, found, and unknown ids", func(t *testing.T) {
		missingReq := httptest.NewRequest(http.MethodGet, "/api/rl/trajectories/", nil)
		missingRec := httptest.NewRecorder()
		rlH.handleGetTrajectory(missingRec, missingReq)
		if missingRec.Code != http.StatusBadRequest {
			t.Fatalf("missing status = %d, want %d", missingRec.Code, http.StatusBadRequest)
		}

		foundReq := httptest.NewRequest(http.MethodGet, "/api/rl/trajectories/gold-1", nil)
		foundReq.SetPathValue("trajectory_id", "gold-1")
		foundRec := httptest.NewRecorder()
		rlH.handleGetTrajectory(foundRec, foundReq)
		if foundRec.Code != http.StatusOK {
			t.Fatalf("found status = %d, want %d", foundRec.Code, http.StatusOK)
		}

		var traj rl.RLTrajectory
		if err := json.Unmarshal(foundRec.Body.Bytes(), &traj); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if traj.ID != "gold-1" {
			t.Fatalf("traj.ID = %q, want gold-1", traj.ID)
		}

		unknownReq := httptest.NewRequest(http.MethodGet, "/api/rl/trajectories/unknown", nil)
		unknownReq.SetPathValue("trajectory_id", "unknown")
		unknownRec := httptest.NewRecorder()
		rlH.handleGetTrajectory(unknownRec, unknownReq)
		if unknownRec.Code != http.StatusNotFound {
			t.Fatalf("unknown status = %d, want %d", unknownRec.Code, http.StatusNotFound)
		}
	})

	t.Run("get config returns current thresholds", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/rl/config", nil)
		rec := httptest.NewRecorder()

		rlH.handleGetConfig(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var cfg rl.QualityConfig
		if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if cfg.GoldMinScore != rlH.config.GoldMinScore {
			t.Fatalf("GoldMinScore = %v, want %v", cfg.GoldMinScore, rlH.config.GoldMinScore)
		}
	})
}

func TestTaskMgmtHandlerUpdateAndDelete(t *testing.T) {
	_, _, taskH := newEvalHTTPTestDeps(t)

	task, err := taskH.manager.Create(task_mgmt.CreateTaskRequest{
		Name:        "Nightly coverage",
		Description: "baseline",
		DatasetPath: "./dataset.json",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("update requires task id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/eval-tasks/", bytes.NewBufferString(`{}`))
		rec := httptest.NewRecorder()

		taskH.handleUpdateTask(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("update persists changes", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPut,
			"/api/eval-tasks/"+task.ID,
			bytes.NewBufferString(`{"name":"Renamed task","status":"archived","metadata":{"owner":"delivery"}}`),
		)
		req.SetPathValue("task_id", task.ID)
		rec := httptest.NewRecorder()

		taskH.handleUpdateTask(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var updated task_mgmt.EvalTaskDefinition
		if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if updated.Name != "Renamed task" || updated.Status != task_mgmt.TaskStatusArchived {
			t.Fatalf("updated task = %#v, want renamed archived task", updated)
		}
		if updated.Metadata["owner"] != "delivery" {
			t.Fatalf("metadata = %#v, want owner override", updated.Metadata)
		}
	})

	t.Run("delete requires task id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/eval-tasks/", nil)
		rec := httptest.NewRecorder()

		taskH.handleDeleteTask(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("delete removes task and follow-up lookup is not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/eval-tasks/"+task.ID, nil)
		req.SetPathValue("task_id", task.ID)
		rec := httptest.NewRecorder()

		taskH.handleDeleteTask(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		getReq := httptest.NewRequest(http.MethodGet, "/api/eval-tasks/"+task.ID, nil)
		getReq.SetPathValue("task_id", task.ID)
		getRec := httptest.NewRecorder()
		taskH.handleGetTask(getRec, getReq)
		if getRec.Code != http.StatusNotFound {
			t.Fatalf("get status = %d, want %d", getRec.Code, http.StatusNotFound)
		}
	})
}

func TestEvalHandlerAdditionalGuards(t *testing.T) {
	evalH, _, _ := newEvalHTTPTestDeps(t)

	t.Run("evaluation lookup requires id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/evaluations/", nil)
		rec := httptest.NewRecorder()

		evalH.handleGetEvaluation(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("delete evaluation requires id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/evaluations/", nil)
		rec := httptest.NewRecorder()

		evalH.handleDeleteEvaluation(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("list agents succeeds on empty store", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
		rec := httptest.NewRecorder()

		evalH.handleListAgents(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var payload struct {
			Agents []any `json:"agents"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if len(payload.Agents) != 0 {
			t.Fatalf("agents = %#v, want empty list", payload.Agents)
		}
	})

	t.Run("agent lookups require id", func(t *testing.T) {
		getReq := httptest.NewRequest(http.MethodGet, "/api/agents/", nil)
		getRec := httptest.NewRecorder()
		evalH.handleGetAgent(getRec, getReq)
		if getRec.Code != http.StatusBadRequest {
			t.Fatalf("get status = %d, want %d", getRec.Code, http.StatusBadRequest)
		}

		listReq := httptest.NewRequest(http.MethodGet, "/api/agents//evaluations", nil)
		listRec := httptest.NewRecorder()
		evalH.handleListAgentEvaluations(listRec, listReq)
		if listRec.Code != http.StatusBadRequest {
			t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusBadRequest)
		}
	})
}
