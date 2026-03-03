package teamrun

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/filestore"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
	id "alex/internal/shared/utils/id"
	"gopkg.in/yaml.v3"
)

const (
	defaultFilePerm = 0o600
)

// FileRecorder persists each team run as an individual YAML file.
type FileRecorder struct {
	baseDir string
	logger  logging.Logger
	clock   func() time.Time
}

type persistedTeamRunRecord struct {
	Version    int                 `yaml:"version"`
	RecordedAt time.Time           `yaml:"recorded_at"`
	Record     agent.TeamRunRecord `yaml:"record"`
}

// NewFileRecorder creates a file-based team run recorder under baseDir.
func NewFileRecorder(baseDir string, logger logging.Logger) (*FileRecorder, error) {
	trimmed := strings.TrimSpace(baseDir)
	if trimmed == "" {
		return nil, fmt.Errorf("team run recorder base dir is required")
	}
	if err := filestore.EnsureDir(trimmed); err != nil {
		return nil, fmt.Errorf("ensure team run recorder dir: %w", err)
	}
	return &FileRecorder{
		baseDir: trimmed,
		logger:  logging.OrNop(logger),
		clock:   time.Now,
	}, nil
}

// RecordTeamRun writes a single team run record and returns the file path.
func (r *FileRecorder) RecordTeamRun(_ context.Context, record agent.TeamRunRecord) (string, error) {
	if r == nil {
		return "", fmt.Errorf("team run recorder is nil")
	}
	runID := strings.TrimSpace(record.RunID)
	if runID == "" {
		runID = id.NewKSUID()
		record.RunID = runID
	}
	recordedAt := r.clock().UTC()
	if record.DispatchedAt.IsZero() {
		record.DispatchedAt = recordedAt
	}
	if utils.IsBlank(record.DispatchState) {
		record.DispatchState = "dispatched"
	}
	payload := persistedTeamRunRecord{
		Version:    1,
		RecordedAt: recordedAt,
		Record:     record,
	}
	data, err := yaml.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal team run record: %w", err)
	}

	safeTeam := sanitizeFileToken(record.TeamName)
	if safeTeam == "" {
		safeTeam = "team"
	}
	filename := fmt.Sprintf(
		"%s-%s-%s.yaml",
		recordedAt.Format("20060102T150405Z"),
		safeTeam,
		runID,
	)
	path := filepath.Join(r.baseDir, filename)
	if err := filestore.AtomicWrite(path, data, defaultFilePerm); err != nil {
		return "", fmt.Errorf("persist team run record: %w", err)
	}

	r.logger.Debug("team run recorded: team=%s run=%s path=%s", record.TeamName, runID, path)
	return path, nil
}

func sanitizeFileToken(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range trimmed {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return ""
	}
	return out
}
