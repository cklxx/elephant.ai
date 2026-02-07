package bootstrap

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"alex/evaluation/rl"
	serverApp "alex/internal/delivery/server/app"
	evalHTTP "alex/internal/delivery/eval/http"
)

// RunEvalServer is the main entry point for the evaluation server.
func RunEvalServer(configPath string) error {
	cfg, err := resolveConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log.Printf("[eval-server] starting (port=%s, env=%s)", cfg.Port, cfg.Environment)

	// Phase 1: Create evaluation service
	evalSvc, err := serverApp.NewEvaluationService(cfg.EvalOutputDir)
	if err != nil {
		return fmt.Errorf("init evaluation service: %w", err)
	}
	log.Printf("[eval-server] evaluation service ready (output=%s)", cfg.EvalOutputDir)

	// Phase 2: Create RL pipeline components
	rlStorage, err := rl.NewStorage(cfg.RLOutputDir)
	if err != nil {
		return fmt.Errorf("init rl storage: %w", err)
	}
	rlExtractor := rl.NewExtractor()
	rlConfig := rl.DefaultQualityConfig()
	qualityGate := rl.NewQualityGate(rlConfig, nil)
	log.Printf("[eval-server] RL pipeline ready (output=%s)", cfg.RLOutputDir)

	// Phase 3: Wire HTTP router
	router := evalHTTP.NewEvalRouter(evalHTTP.EvalRouterDeps{
		Evaluation:  evalSvc,
		RLStorage:   rlStorage,
		RLExtractor: rlExtractor,
		QualityGate: qualityGate,
		RLConfig:    rlConfig,
	}, evalHTTP.EvalRouterConfig{
		Environment:    cfg.Environment,
		AllowedOrigins: cfg.AllowedOrigins,
		RLOutputDir:    cfg.RLOutputDir,
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Phase 4: Graceful shutdown
	errCh := make(chan error, 1)
	go func() {
		log.Printf("[eval-server] listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("[eval-server] received signal %s, shutting down...", sig)
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	log.Println("[eval-server] stopped")
	return nil
}

func resolveConfig(path string) (*EvalServerConfig, error) {
	if path != "" {
		return LoadConfig(path)
	}
	candidates := []string{
		"configs/eval-server.yaml",
		"eval-server.yaml",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return LoadConfig(c)
		}
	}
	return DefaultConfig(), nil
}
