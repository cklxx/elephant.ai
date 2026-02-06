package http

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"alex/internal/memory"
)

type MemoryDailyEntry struct {
	Date    string `json:"date"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

type MemorySnapshot struct {
	UserID   string             `json:"user_id"`
	LongTerm string             `json:"long_term"`
	Daily    []MemoryDailyEntry `json:"daily"`
}

// HandleGetMemorySnapshot handles GET /api/dev/memory.
func (h *APIHandler) HandleGetMemorySnapshot(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.NotFound(w, r)
		return
	}
	if h.memoryEngine == nil {
		h.writeJSONError(w, http.StatusNotFound, "memory engine not configured", nil)
		return
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	session, err := h.coordinator.GetSession(r.Context(), sessionID)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "Session not found", err)
		return
	}

	userID := strings.TrimSpace(session.Metadata["user_id"])
	root := memory.ResolveUserRoot(h.memoryEngine.RootDir(), userID)
	if root == "" {
		h.writeJSONError(w, http.StatusNotFound, "Memory root not configured", nil)
		return
	}

	longTerm, err := h.memoryEngine.LoadLongTerm(r.Context(), userID)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to load memory", err)
		return
	}

	daily, err := loadDailySnapshot(root)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to load daily memory", err)
		return
	}

	h.writeJSON(w, http.StatusOK, MemorySnapshot{
		UserID:   userID,
		LongTerm: longTerm,
		Daily:    daily,
	})
}

func loadDailySnapshot(root string) ([]MemoryDailyEntry, error) {
	dailyDir := filepath.Join(root, "memory")
	entries, err := os.ReadDir(dailyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		files = append(files, entry.Name())
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})

	daily := make([]MemoryDailyEntry, 0, len(files))
	for _, name := range files {
		path := filepath.Join(dailyDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		date := strings.TrimSuffix(name, ".md")
		relPath := filepath.ToSlash(filepath.Join("memory", name))
		daily = append(daily, MemoryDailyEntry{
			Date:    date,
			Path:    relPath,
			Content: strings.TrimSpace(string(content)),
		})
	}

	return daily, nil
}
