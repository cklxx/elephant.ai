package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	agent_eval "alex/evaluation/agent_eval"
	"alex/internal/utils"
)

// EvaluationService runs and tracks agent evaluation jobs for the web surface.
type EvaluationService struct {
	manager       *agent_eval.EvaluationManager
	defaultConfig agent_eval.EvaluationConfig
	logger        *utils.Logger
	baseOutputDir string
}

// NewEvaluationService constructs a new evaluation service with sane defaults.
func NewEvaluationService(baseOutputDir string) (*EvaluationService, error) {
	if baseOutputDir == "" {
		baseOutputDir = "./evaluation_results"
	}
	baseOutputDir = filepath.Clean(baseOutputDir)
	if err := ensureSafeBaseDir(baseOutputDir); err != nil {
		return nil, err
	}

	defaultConfig := agent_eval.EvaluationConfig{
		DatasetType:    "swe_bench",
		DatasetPath:    "./evaluation/swe_bench/real_instances.json",
		InstanceLimit:  10,
		MaxWorkers:     2,
		AgentID:        "default-agent",
		TimeoutPerTask: 5 * time.Minute,
		EnableMetrics:  true,
		MetricsTypes:   []string{"performance", "quality", "resource", "behavior"},
		OutputDir:      baseOutputDir,
		ReportFormat:   "markdown",
	}

	if err := os.MkdirAll(baseOutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create evaluation output dir: %w", err)
	}

	svc := &EvaluationService{
		manager:       agent_eval.NewEvaluationManager(&defaultConfig),
		defaultConfig: defaultConfig,
		logger:        utils.NewComponentLogger("EvaluationService"),
		baseOutputDir: baseOutputDir,
	}

	_ = svc.manager.HydrateFromStore()

	return svc, nil
}

// Start kicks off an evaluation job based on the provided options.
func (s *EvaluationService) Start(ctx context.Context, options *agent_eval.EvaluationOptions) (*agent_eval.EvaluationJob, error) {
	_ = s.manager.HydrateFromStore()

	if options == nil {
		options = agent_eval.DefaultEvaluationOptions()
	}

	config := s.mergeOptions(options)
	if err := agent_eval.ValidateConfig(config); err != nil {
		return nil, err
	}
	if err := s.ensureOutputDir(config.OutputDir); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(config.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output dir %s: %w", config.OutputDir, err)
	}

	s.logger.Info("Scheduling evaluation: dataset=%s limit=%d workers=%d", config.DatasetPath, config.InstanceLimit, config.MaxWorkers)
	return s.manager.ScheduleEvaluation(ctx, config)
}

// ListJobs returns snapshots for all known evaluation jobs.
func (s *EvaluationService) ListJobs() []*agent_eval.EvaluationJob {
	return s.manager.ListJobs()
}

// GetJob returns the snapshot for a specific evaluation job.
func (s *EvaluationService) GetJob(jobID string) (*agent_eval.EvaluationJob, error) {
	return s.manager.GetJob(jobID)
}

// GetJobResults returns the computed results for a job.
func (s *EvaluationService) GetJobResults(jobID string) (*agent_eval.EvaluationResults, error) {
	return s.manager.GetJobResults(jobID)
}

// DeleteEvaluation removes a persisted evaluation snapshot and evicts cached state.
func (s *EvaluationService) DeleteEvaluation(jobID string) error {
	return s.manager.DeleteEvaluation(jobID)
}

// GetAgentProfile returns the persisted profile for the provided agent ID.
func (s *EvaluationService) GetAgentProfile(agentID string) (*agent_eval.AgentProfile, error) {
	return s.manager.GetAgentProfile(agentID)
}

// ListAgentProfiles returns all persisted agent profiles.
func (s *EvaluationService) ListAgentProfiles() ([]*agent_eval.AgentProfile, error) {
	return s.manager.ListAgentProfiles()
}

// ListAgentEvaluations returns stored evaluation snapshots for a given agent.
func (s *EvaluationService) ListAgentEvaluations(agentID string) ([]*agent_eval.EvaluationResults, error) {
	return s.manager.ListAgentEvaluations(agentID)
}

// ListAllEvaluations returns stored evaluation snapshots across agents, limited if requested.
func (s *EvaluationService) ListAllEvaluations(limit int) ([]*agent_eval.EvaluationResults, error) {
	return s.manager.ListAllEvaluations(limit)
}

// QueryEvaluations returns stored evaluation snapshots filtered by the provided query.
func (s *EvaluationService) QueryEvaluations(query agent_eval.EvaluationQuery) ([]*agent_eval.EvaluationResults, error) {
	return s.manager.QueryEvaluations(query)
}

func (s *EvaluationService) mergeOptions(options *agent_eval.EvaluationOptions) *agent_eval.EvaluationConfig {
	config := s.defaultConfig

	if options.DatasetPath != "" {
		config.DatasetPath = options.DatasetPath
	}
	if options.InstanceLimit > 0 {
		config.InstanceLimit = options.InstanceLimit
	}
	if options.MaxWorkers > 0 {
		config.MaxWorkers = options.MaxWorkers
	}
	if options.TimeoutPerTask > 0 {
		config.TimeoutPerTask = options.TimeoutPerTask
	}
	if options.OutputDir != "" {
		config.OutputDir = options.OutputDir
	}
	if options.AgentID != "" {
		config.AgentID = options.AgentID
	}
	if !options.EnableMetrics {
		config.EnableMetrics = false
	}
	if options.ReportFormat != "" {
		config.ReportFormat = options.ReportFormat
	}

	// Store outputs per job under a deterministic folder rooted in the service's base directory.
	config.OutputDir = s.safeOutputDir(config.OutputDir)

	return &config
}

func (s *EvaluationService) safeOutputDir(requested string) string {
	cleaned := filepath.Clean(requested)
	if filepath.IsAbs(cleaned) {
		cleaned = strings.TrimPrefix(cleaned, string(filepath.Separator))
	}

	joined := filepath.Join(s.baseOutputDir, cleaned)
	if rel, err := filepath.Rel(s.baseOutputDir, joined); err == nil {
		if strings.HasPrefix(rel, "..") {
			return s.baseOutputDir
		}
	}

	return joined
}

func (s *EvaluationService) ensureOutputDir(outputDir string) error {
	cleaned := filepath.Clean(outputDir)
	rel, err := filepath.Rel(s.baseOutputDir, cleaned)
	if err != nil {
		return fmt.Errorf("invalid output dir: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("output dir %s escapes base dir", outputDir)
	}
	return nil
}

func ensureSafeBaseDir(baseOutputDir string) error {
	cleaned := filepath.Clean(baseOutputDir)
	if cleaned == ".." || cleaned == "." {
		return fmt.Errorf("base output dir must not resolve to parent or current directory: %s", baseOutputDir)
	}
	if strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("base output dir must not start with parent reference: %s", baseOutputDir)
	}
	if strings.Contains(cleaned, string(filepath.Separator)+".."+string(filepath.Separator)) {
		return fmt.Errorf("base output dir must not contain parent references: %s", baseOutputDir)
	}
	return nil
}
