// alex-server is the Lark-primary binary.  It runs the Lark WebSocket gateway
// with an embedded lightweight debug HTTP server on :9090 (configurable via
// ALEX_DEBUG_PORT or server.debug_port in config.yaml).
//
// The full web API (auth, sessions, tasks, evaluations, frontend) has moved to
// the alex-web binary (cmd/alex-web).
package main

import (
	"log"
	"os"

	serverBootstrap "alex/internal/delivery/server/bootstrap"
	runtimeconfig "alex/internal/shared/config"
)

type runners struct {
	runLark       func(string) error
	runKernelOnce func(string) error
}

func run(args []string, obsConfig string, logger *log.Logger, rs runners) error {
	if logger == nil {
		logger = log.Default()
	}
	if rs.runLark == nil {
		rs.runLark = serverBootstrap.RunLark
	}
	if rs.runKernelOnce == nil {
		rs.runKernelOnce = serverBootstrap.RunKernelOnce
	}

	if len(args) > 1 {
		switch args[1] {
		case "lark":
			// Backward compatibility for legacy scripts (`alex-server lark`).
			logger.Printf("DEPRECATED: 'alex-server lark' is no longer needed — alex-server always runs in Lark mode. " +
				"Please update your scripts to just run 'alex-server'.")
			return rs.runLark(obsConfig)
		case "kernel-once":
			return rs.runKernelOnce(obsConfig)
		}
	}
	return rs.runLark(obsConfig)
}

func main() {
	if err := runtimeconfig.LoadDotEnv(); err != nil {
		log.Printf("Warning: failed to load .env: %v", err)
	}

	obsConfig := os.Getenv("ALEX_OBSERVABILITY_CONFIG")
	if err := run(os.Args, obsConfig, log.Default(), runners{}); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
