package http

import (
	"encoding/json"
	"net/http"
	"time"

	"alex/evaluation/rl"
)

// rlHandler holds RL-specific HTTP handler state.
type rlHandler struct {
	storage     *rl.Storage
	extractor   *rl.Extractor
	qualityGate *rl.QualityGate
	config      rl.QualityConfig
}

func (h *rlHandler) handleGetStats(w http.ResponseWriter, _ *http.Request) {
	if h.storage == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "RL storage not configured")
		return
	}
	manifest, err := h.storage.Stats()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to read RL stats")
		return
	}
	writeJSON(w, http.StatusOK, manifest)
}

func (h *rlHandler) handleListTrajectories(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "RL storage not configured")
		return
	}

	tier := rl.QualityTier(r.URL.Query().Get("tier"))
	if tier == "" {
		tier = rl.TierGold
	}

	var after, before time.Time
	if afterStr := r.URL.Query().Get("after"); afterStr != "" {
		if t, err := time.Parse("2006-01-02", afterStr); err == nil {
			after = t
		}
	}
	if beforeStr := r.URL.Query().Get("before"); beforeStr != "" {
		if t, err := time.Parse("2006-01-02", beforeStr); err == nil {
			before = t
		}
	}

	trajectories, err := h.storage.ReadTier(tier, after, before)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to read trajectories")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tier":         tier,
		"count":        len(trajectories),
		"trajectories": trajectories,
	})
}

func (h *rlHandler) handleGetTrajectory(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "RL storage not configured")
		return
	}

	id := r.PathValue("trajectory_id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "trajectory_id is required")
		return
	}

	// Search across all tiers
	for _, tier := range rl.ValidTiers {
		trajectories, err := h.storage.ReadTier(tier, time.Time{}, time.Time{})
		if err != nil {
			continue
		}
		for _, traj := range trajectories {
			if traj.ID == id {
				writeJSON(w, http.StatusOK, traj)
				return
			}
		}
	}

	writeJSONError(w, http.StatusNotFound, "Trajectory not found")
}

func (h *rlHandler) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.config)
}

func (h *rlHandler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var cfg rl.QualityConfig
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&cfg); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid config body")
		return
	}
	h.config = cfg
	h.qualityGate = rl.NewQualityGate(cfg, nil) // Judge re-wired in Batch 8
	writeJSON(w, http.StatusOK, h.config)
}

type exportRequest struct {
	Tier  string `json:"tier"`
	After string `json:"after,omitempty"`
}

func (h *rlHandler) handleExport(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "RL storage not configured")
		return
	}

	tier := rl.QualityTier(r.URL.Query().Get("tier"))
	if tier == "" {
		tier = rl.TierGold
	}

	var after time.Time
	if afterStr := r.URL.Query().Get("after"); afterStr != "" {
		if t, err := time.Parse("2006-01-02", afterStr); err == nil {
			after = t
		}
	}

	trajectories, err := h.storage.ReadTier(tier, after, time.Time{})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to read trajectories for export")
		return
	}

	w.Header().Set("Content-Type", "application/jsonl")
	w.Header().Set("Content-Disposition", "attachment; filename="+string(tier)+"_trajectories.jsonl")
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	for _, traj := range trajectories {
		enc.Encode(traj)
	}
}
