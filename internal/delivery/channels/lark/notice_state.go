package lark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/shared/logging"
)

const (
	noticeStatePathEnv  = "LARK_NOTICE_STATE_FILE"
	noticeStateFileName = "lark-notice.state.json"
)

// NoticeBinding records which Lark chat should receive supervisor notices.
type NoticeBinding struct {
	ChatID      string `json:"chat_id"`
	SetByUserID string `json:"set_by_user_id,omitempty"`
	SetByName   string `json:"set_by_name,omitempty"`
	SetAt       string `json:"set_at"`
	UpdatedAt   string `json:"updated_at"`
}

type noticeStateStore struct {
	path   string
	logger logging.Logger
	now    func() time.Time
}

func newNoticeStateStore(logger logging.Logger) *noticeStateStore {
	return &noticeStateStore{
		path:   resolveNoticeStatePath(os.Getenv, os.Getwd),
		logger: logging.OrNop(logger),
		now:    time.Now,
	}
}

func resolveNoticeStatePath(getenv func(string) string, getwd func() (string, error)) string {
	if getenv != nil {
		if explicit := strings.TrimSpace(getenv(noticeStatePathEnv)); explicit != "" {
			return explicit
		}
	}

	workingDir := "."
	if getwd != nil {
		if wd, err := getwd(); err == nil && strings.TrimSpace(wd) != "" {
			workingDir = wd
		}
	}

	mainRoot := inferMainRootFromWorkingDir(workingDir)
	return filepath.Join(mainRoot, ".worktrees", "test", "tmp", noticeStateFileName)
}

func inferMainRootFromWorkingDir(workingDir string) string {
	cleaned := filepath.Clean(strings.TrimSpace(workingDir))
	if cleaned == "" {
		return "."
	}

	suffix := string(filepath.Separator) + filepath.Join(".worktrees", "test")
	if idx := strings.LastIndex(cleaned, suffix); idx > 0 {
		return cleaned[:idx]
	}
	if strings.HasSuffix(cleaned, filepath.Join(".worktrees", "test")) {
		if parent := filepath.Dir(filepath.Dir(cleaned)); strings.TrimSpace(parent) != "" {
			return parent
		}
	}
	return cleaned
}

func (s *noticeStateStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *noticeStateStore) Save(chatID, setByUserID, setByName string) (NoticeBinding, error) {
	if s == nil {
		return NoticeBinding{}, fmt.Errorf("notice state store is nil")
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return NoticeBinding{}, fmt.Errorf("chat_id is required")
	}

	now := s.now
	if now == nil {
		now = time.Now
	}
	nowUTC := now().UTC().Format(time.RFC3339)
	binding := NoticeBinding{
		ChatID:      chatID,
		SetByUserID: strings.TrimSpace(setByUserID),
		SetByName:   strings.TrimSpace(setByName),
		SetAt:       nowUTC,
		UpdatedAt:   nowUTC,
	}
	if previous, ok, err := s.Load(); err == nil && ok {
		if previous.ChatID == chatID && strings.TrimSpace(previous.SetAt) != "" {
			binding.SetAt = previous.SetAt
		}
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return NoticeBinding{}, fmt.Errorf("mkdir notice state dir: %w", err)
	}

	payload, err := json.MarshalIndent(binding, "", "  ")
	if err != nil {
		return NoticeBinding{}, fmt.Errorf("marshal notice state: %w", err)
	}
	payload = append(payload, '\n')

	tmpFile := s.path + ".tmp"
	if err := os.WriteFile(tmpFile, payload, 0o644); err != nil {
		return NoticeBinding{}, fmt.Errorf("write notice state tmp: %w", err)
	}
	if err := os.Rename(tmpFile, s.path); err != nil {
		return NoticeBinding{}, fmt.Errorf("rename notice state: %w", err)
	}
	return binding, nil
}

func (s *noticeStateStore) Load() (NoticeBinding, bool, error) {
	if s == nil {
		return NoticeBinding{}, false, fmt.Errorf("notice state store is nil")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return NoticeBinding{}, false, nil
		}
		return NoticeBinding{}, false, fmt.Errorf("read notice state: %w", err)
	}

	var binding NoticeBinding
	if err := json.Unmarshal(data, &binding); err != nil {
		return NoticeBinding{}, false, fmt.Errorf("decode notice state: %w", err)
	}
	binding.ChatID = strings.TrimSpace(binding.ChatID)
	if binding.ChatID == "" {
		return NoticeBinding{}, false, nil
	}
	binding.SetByUserID = strings.TrimSpace(binding.SetByUserID)
	binding.SetByName = strings.TrimSpace(binding.SetByName)
	binding.SetAt = strings.TrimSpace(binding.SetAt)
	binding.UpdatedAt = strings.TrimSpace(binding.UpdatedAt)
	return binding, true, nil
}

func (s *noticeStateStore) Clear() error {
	if s == nil {
		return fmt.Errorf("notice state store is nil")
	}
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove notice state: %w", err)
	}
	return nil
}
