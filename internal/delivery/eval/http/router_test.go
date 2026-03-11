package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alex/evaluation/rl"
	"alex/evaluation/task_mgmt"
	serverApp "alex/internal/delivery/server/app"
)

func TestNewEvalRouterHealthAndCORS(t *testing.T) {
	router := NewEvalRouter(EvalRouterDeps{}, EvalRouterConfig{
		Environment:    "production",
		AllowedOrigins: []string{"https://console.example"},
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://console.example")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://console.example" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want allowed origin", got)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload["status"] != "ok" || payload["service"] != "eval-server" {
		t.Fatalf("payload = %#v, want eval health response", payload)
	}
}

func TestNewEvalRouterDependencyGuards(t *testing.T) {
	router := NewEvalRouter(EvalRouterDeps{}, EvalRouterConfig{Environment: "development"})

	tests := []struct {
		name    string
		method  string
		target  string
		body    string
		status  int
		message string
	}{
		{
			name:    "evaluation unavailable",
			method:  http.MethodGet,
			target:  "/api/evaluations",
			status:  http.StatusServiceUnavailable,
			message: "Evaluation service unavailable",
		},
		{
			name:    "rl storage unavailable",
			method:  http.MethodGet,
			target:  "/api/rl/stats",
			status:  http.StatusServiceUnavailable,
			message: "RL storage not configured",
		},
		{
			name:    "task manager unavailable",
			method:  http.MethodGet,
			target:  "/api/eval-tasks",
			status:  http.StatusServiceUnavailable,
			message: "Task management not configured",
		},
		{
			name:    "invalid rl config body",
			method:  http.MethodPut,
			target:  "/api/rl/config",
			body:    "{",
			status:  http.StatusBadRequest,
			message: "Invalid config body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}

			var payload map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if payload["error"] != tt.message {
				t.Fatalf("error = %q, want %q", payload["error"], tt.message)
			}
		})
	}
}

func TestNewEvalRouterCriticalPaths(t *testing.T) {
	evalSvc, err := serverApp.NewEvaluationService(t.TempDir())
	if err != nil {
		t.Fatalf("NewEvaluationService() error = %v", err)
	}
	rlStorage, err := rl.NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorage() error = %v", err)
	}
	store, err := task_mgmt.NewTaskStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTaskStore() error = %v", err)
	}
	taskManager := task_mgmt.NewTaskManager(store)
	router := NewEvalRouter(EvalRouterDeps{
		Evaluation:  evalSvc,
		RLStorage:   rlStorage,
		QualityGate: rl.NewQualityGate(rl.DefaultQualityConfig(), nil),
		RLConfig:    rl.DefaultQualityConfig(),
		TaskManager: taskManager,
	}, EvalRouterConfig{Environment: "development"})

	t.Run("evaluation invalid body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/evaluations", strings.NewReader("{"))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("rl config update", func(t *testing.T) {
		body := bytes.NewBufferString(`{"gold_min_score": 95, "silver_min_score": 75, "bronze_min_score": 50}`)
		req := httptest.NewRequest(http.MethodPut, "/api/rl/config", body)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var cfg rl.QualityConfig
		if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if cfg.GoldMinScore != 95 || cfg.SilverMinScore != 75 || cfg.BronzeMinScore != 50 {
			t.Fatalf("cfg = %#v, want updated thresholds", cfg)
		}
	})

	t.Run("task create and run", func(t *testing.T) {
		createReq := bytes.NewBufferString(`{"name":"Nightly sweep","dataset_path":"./dataset.json"}`)
		createHTTPReq := httptest.NewRequest(http.MethodPost, "/api/eval-tasks", createReq)
		createRec := httptest.NewRecorder()
		router.ServeHTTP(createRec, createHTTPReq)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("create status = %d, want %d", createRec.Code, http.StatusCreated)
		}

		var task task_mgmt.EvalTaskDefinition
		if err := json.Unmarshal(createRec.Body.Bytes(), &task); err != nil {
			t.Fatalf("Unmarshal(create) error = %v", err)
		}
		if task.Name != "Nightly sweep" {
			t.Fatalf("task.Name = %q, want Nightly sweep", task.Name)
		}

		getReq := httptest.NewRequest(http.MethodGet, "/api/eval-tasks/"+task.ID, nil)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("get status = %d, want %d", getRec.Code, http.StatusOK)
		}

		runReq := httptest.NewRequest(http.MethodPost, "/api/eval-tasks/"+task.ID+"/run", nil)
		runRec := httptest.NewRecorder()
		router.ServeHTTP(runRec, runReq)
		if runRec.Code != http.StatusAccepted {
			t.Fatalf("run status = %d, want %d", runRec.Code, http.StatusAccepted)
		}

		var run task_mgmt.BatchRun
		if err := json.Unmarshal(runRec.Body.Bytes(), &run); err != nil {
			t.Fatalf("Unmarshal(run) error = %v", err)
		}
		if run.TaskID != task.ID || run.Status != task_mgmt.RunStatusPending || run.StartedAt.IsZero() {
			t.Fatalf("run = %#v, want pending run for created task", run)
		}
	})

	t.Run("rl export", func(t *testing.T) {
		traj := &rl.RLTrajectory{
			ID:          "traj-1",
			QualityTier: rl.TierGold,
			ExtractedAt: time.Date(2026, time.March, 11, 10, 0, 0, 0, time.UTC),
		}
		if err := rlStorage.Append(traj); err != nil {
			t.Fatalf("Append() error = %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/rl/export?tier=gold", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/jsonl" {
			t.Fatalf("Content-Type = %q, want application/jsonl", got)
		}
		if !strings.Contains(rec.Body.String(), "\"id\":\"traj-1\"") {
			t.Fatalf("body = %q, want exported trajectory", rec.Body.String())
		}
	})
}
