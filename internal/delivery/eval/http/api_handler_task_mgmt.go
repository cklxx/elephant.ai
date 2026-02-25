package http

import (
	"encoding/json"
	"net/http"

	"alex/evaluation/task_mgmt"
)

// taskMgmtHandler holds task management HTTP handler state.
type taskMgmtHandler struct {
	manager *task_mgmt.TaskManager
}

func (h *taskMgmtHandler) handleListTasks(w http.ResponseWriter, _ *http.Request) {
	if h.manager == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Task management not configured")
		return
	}
	tasks, err := h.manager.List()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list tasks")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (h *taskMgmtHandler) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Task management not configured")
		return
	}
	var req task_mgmt.CreateTaskRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<18)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	task, err := h.manager.Create(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (h *taskMgmtHandler) handleGetTask(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Task management not configured")
		return
	}
	id := r.PathValue("task_id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "task_id is required")
		return
	}
	task, err := h.manager.Get(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Task not found")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (h *taskMgmtHandler) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Task management not configured")
		return
	}
	id := r.PathValue("task_id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "task_id is required")
		return
	}
	var req task_mgmt.UpdateTaskRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<18)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	task, err := h.manager.Update(id, req)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (h *taskMgmtHandler) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Task management not configured")
		return
	}
	id := r.PathValue("task_id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "task_id is required")
		return
	}
	if err := h.manager.Delete(id); err != nil {
		writeJSONError(w, http.StatusNotFound, "Task not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func (h *taskMgmtHandler) handleRunTask(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Task management not configured")
		return
	}
	id := r.PathValue("task_id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "task_id is required")
		return
	}
	// Record the run intent; actual execution delegated to evaluation service
	run, err := h.manager.RecordRun(id, "")
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}
