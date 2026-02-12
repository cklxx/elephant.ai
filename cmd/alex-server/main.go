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

func main() {
	if err := runtimeconfig.LoadDotEnv(); err != nil {
		log.Printf("Warning: failed to load .env: %v", err)
	}

	obsConfig := os.Getenv("ALEX_OBSERVABILITY_CONFIG")

	// Backward compatibility: ignore legacy "lark" subcommand so existing
	// scripts like `alex-server lark` continue to work during transition.
	if len(os.Args) > 1 && os.Args[1] == "lark" {
		log.Printf("DEPRECATED: 'alex-server lark' is no longer needed — alex-server always runs in Lark mode. " +
			"Please update your scripts to just run 'alex-server'.")
	}

	if err := serverBootstrap.RunLark(obsConfig); err != nil {
		log.Fatalf("lark mode exited: %v", err)
	}
}
