package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	ctxconfig "alex/internal/app/context"
	core "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"gopkg.in/yaml.v3"
)

var contextConfigSections = []string{
	"personas",
	"goals",
	"policies",
	"knowledge",
	"worlds",
}

var contextConfigSectionSet = func() map[string]struct{} {
	out := make(map[string]struct{}, len(contextConfigSections))
	for _, section := range contextConfigSections {
		out[section] = struct{}{}
	}
	return out
}()

type ContextConfigHandler struct {
	root string
}

type ContextConfigFile struct {
	Path      string    `json:"path"`
	Section   string    `json:"section"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ContextConfigResponse struct {
	Root  string              `json:"root"`
	Files []ContextConfigFile `json:"files"`
}

type ContextConfigUpdateRequest struct {
	Files []ContextConfigUpdate `json:"files"`
}

type ContextConfigUpdate struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func NewContextConfigHandler(root string) *ContextConfigHandler {
	if strings.TrimSpace(root) == "" {
		root = ctxconfig.ResolveConfigRoot()
	}
	root = filepath.Clean(root)
	return &ContextConfigHandler{root: root}
}

func (h *ContextConfigHandler) HandleContextConfig(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.HandleGetContextConfig(w, r)
	case http.MethodPut:
		h.HandleUpdateContextConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ContextConfigHandler) HandleContextPreview(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	toolMode := strings.TrimSpace(query.Get("tool_mode"))
	if toolMode == "" {
		toolMode = "web"
	}
	toolPreset := strings.TrimSpace(query.Get("tool_preset"))

	cfg := agent.ContextWindowConfig{
		PersonaKey: strings.TrimSpace(query.Get("persona_key")),
		GoalKey:    strings.TrimSpace(query.Get("goal_key")),
		WorldKey:   strings.TrimSpace(query.Get("world_key")),
		ToolMode:   toolMode,
		ToolPreset: toolPreset,
	}

	manager := ctxconfig.NewManager(ctxconfig.WithConfigRoot(h.root))
	session := &storage.Session{ID: "context-preview", Messages: nil}
	window, err := manager.BuildWindow(r.Context(), session, cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	estimateMessages := append([]core.Message(nil), window.Messages...)
	if strings.TrimSpace(window.SystemPrompt) != "" {
		estimateMessages = append(estimateMessages, core.Message{
			Role:    "system",
			Content: window.SystemPrompt,
			Source:  core.MessageSourceSystemPrompt,
		})
	}
	tokenEstimate := manager.EstimateTokens(estimateMessages)
	if tokenEstimate == 0 {
		tokenEstimate = estimateTokensFromText(window.SystemPrompt)
	}

	response := ContextWindowPreviewResponse{
		SessionID:     session.ID,
		TokenEstimate: tokenEstimate,
		TokenLimit:    cfg.TokenLimit,
		PersonaKey:    cfg.PersonaKey,
		ToolMode:      cfg.ToolMode,
		ToolPreset:    cfg.ToolPreset,
		Window:        window,
	}
	writeJSON(w, http.StatusOK, response)
}

func estimateTokensFromText(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	return len([]rune(trimmed)) / 4
}

func (h *ContextConfigHandler) HandleGetContextConfig(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	files, err := h.loadContextFiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, ContextConfigResponse{
		Root:  h.root,
		Files: files,
	})
}

func (h *ContextConfigHandler) HandleUpdateContextConfig(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	var body ContextConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}
	if len(body.Files) == 0 {
		http.Error(w, "no context files provided", http.StatusBadRequest)
		return
	}

	updates := make(map[string]ContextConfigUpdate, len(body.Files))
	for _, file := range body.Files {
		cleanPath, _, err := h.validateUpdatePath(file.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validateYAML(file.Content); err != nil {
			http.Error(w, fmt.Sprintf("invalid YAML for %s: %v", cleanPath, err), http.StatusBadRequest)
			return
		}
		updates[cleanPath] = ContextConfigUpdate{
			Path:    cleanPath,
			Content: file.Content,
		}
	}

	for _, update := range updates {
		target := filepath.Join(h.root, update.Path)
		if err := writeFileAtomic(target, []byte(update.Content)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	files, err := h.loadContextFiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, ContextConfigResponse{
		Root:  h.root,
		Files: files,
	})
}

func (h *ContextConfigHandler) loadContextFiles() ([]ContextConfigFile, error) {
	if h == nil {
		return nil, errors.New("context config handler not configured")
	}
	if strings.TrimSpace(h.root) == "" {
		return nil, errors.New("context config root missing")
	}
	info, err := os.Stat(h.root)
	if err != nil {
		return nil, fmt.Errorf("context root missing: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("context root is not a directory: %s", h.root)
	}

	files := make([]ContextConfigFile, 0)
	for _, section := range contextConfigSections {
		dir := filepath.Join(h.root, section)
		sectionInfo, err := os.Stat(dir)
		if err != nil {
			return nil, fmt.Errorf("context directory missing: %s", dir)
		}
		if !sectionInfo.IsDir() {
			return nil, fmt.Errorf("context directory is not a folder: %s", dir)
		}

		walkErr := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !isYAMLFile(entry.Name()) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(h.root, path)
			if err != nil {
				return err
			}
			files = append(files, ContextConfigFile{
				Path:      filepath.ToSlash(rel),
				Section:   section,
				Name:      entry.Name(),
				Content:   string(data),
				UpdatedAt: info.ModTime().UTC(),
			})
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func (h *ContextConfigHandler) validateUpdatePath(path string) (string, string, error) {
	if h == nil {
		return "", "", errors.New("context config handler not configured")
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", "", errors.New("context path required")
	}
	if filepath.IsAbs(trimmed) {
		return "", "", errors.New("context path must be relative")
	}
	clean := filepath.Clean(filepath.FromSlash(trimmed))
	if clean == "." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
		return "", "", errors.New("context path escapes root")
	}
	slash := filepath.ToSlash(clean)
	parts := strings.Split(slash, "/")
	if len(parts) < 2 {
		return "", "", errors.New("context path must include a section and filename")
	}
	section := parts[0]
	if _, ok := contextConfigSectionSet[section]; !ok {
		return "", "", fmt.Errorf("unsupported context section: %s", section)
	}
	name := parts[len(parts)-1]
	if !isYAMLFile(name) {
		return "", "", errors.New("context file must be .yaml or .yml")
	}
	target := filepath.Join(h.root, clean)
	rel, err := filepath.Rel(h.root, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", "", errors.New("context path escapes root")
	}
	return clean, section, nil
}

func isYAMLFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}

func validateYAML(content string) error {
	var payload any
	if err := yaml.Unmarshal([]byte(content), &payload); err != nil {
		return err
	}
	return nil
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".context-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	shouldCleanup := true
	defer func() {
		if shouldCleanup {
			_ = os.Remove(tempName)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	shouldCleanup = false
	return nil
}
