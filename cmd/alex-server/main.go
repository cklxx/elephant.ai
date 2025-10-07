package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"alex/internal/di"
	serverApp "alex/internal/server/app"
	serverHTTP "alex/internal/server/http"
	"alex/internal/utils"
)

// Config holds server configuration
type Config struct {
	LLMProvider   string
	LLMModel      string
	APIKey        string
	BaseURL       string
	MaxTokens     int
	MaxIterations int
	Port          string
}

func main() {
	logger := utils.NewComponentLogger("Main")
	logger.Info("Starting ALEX SSE Server...")

	// Load configuration
	config := loadConfig()

	// Log configuration for debugging
	logger.Info("=== Server Configuration ===")
	logger.Info("LLM Provider: %s", config.LLMProvider)
	logger.Info("LLM Model: %s", config.LLMModel)
	logger.Info("Base URL: %s", config.BaseURL)
	logger.Info("API Key: %s...%s", config.APIKey[:10], config.APIKey[len(config.APIKey)-10:])
	logger.Info("Max Tokens: %d", config.MaxTokens)
	logger.Info("Max Iterations: %d", config.MaxIterations)
	logger.Info("Port: %s", config.Port)
	logger.Info("===========================")

	// Initialize container
	container, err := buildContainer(config)
	if err != nil {
		log.Fatalf("Failed to initialize container: %v", err)
	}
	defer func() {
		if err := container.Cleanup(); err != nil {
			log.Printf("Failed to cleanup container: %v", err)
		}
	}()

	// Create server coordinator
	broadcaster := serverApp.NewEventBroadcaster()
	taskStore := serverApp.NewInMemoryTaskStore()

	// Set task store on broadcaster for progress tracking
	broadcaster.SetTaskStore(taskStore)

	serverCoordinator := serverApp.NewServerCoordinator(
		container.AgentCoordinator,
		broadcaster,
		container.SessionStore,
		taskStore,
	)

	// Setup HTTP router
	router := serverHTTP.NewRouter(serverCoordinator, broadcaster)

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // No timeout for SSE
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Server listening on :%s", config.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server stopped")
}

// buildContainer builds the dependency injection container
func buildContainer(config Config) (*di.Container, error) {
	// Build DI container with configurable storage
	diConfig := di.Config{
		LLMProvider:   config.LLMProvider,
		LLMModel:      config.LLMModel,
		APIKey:        config.APIKey,
		BaseURL:       config.BaseURL,
		MaxTokens:     config.MaxTokens,
		MaxIterations: config.MaxIterations,
		SessionDir:    di.GetStorageDir("ALEX_SESSION_DIR", "~/.alex-sessions"),
		CostDir:       di.GetStorageDir("ALEX_COST_DIR", "~/.alex-costs"),
	}

	return di.BuildContainer(diConfig)
}

// loadConfig loads configuration from environment variables
func loadConfig() Config {
	provider := getEnv("ALEX_LLM_PROVIDER", "openai")

	config := Config{
		LLMProvider:   provider,
		LLMModel:      getEnv("ALEX_LLM_MODEL", "gpt-4o"),
		APIKey:        di.GetAPIKey(provider),
		BaseURL:       getEnv("ALEX_BASE_URL", ""),
		MaxTokens:     128000,
		MaxIterations: 20,
		Port:          getEnv("PORT", "8080"),
	}

	// Validate required configuration (skip for Ollama which doesn't need API key)
	if config.APIKey == "" && provider != "ollama" {
		fmt.Fprintf(os.Stderr, "Error: API key required for provider '%s'\n", provider)
		fmt.Fprintf(os.Stderr, "Set one of: OPENAI_API_KEY, OPENROUTER_API_KEY, DEEPSEEK_API_KEY\n")
		os.Exit(1)
	}

	return config
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
