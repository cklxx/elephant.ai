// alex-web is the standalone web API + frontend binary.
// It runs the full HTTP server with auth, sessions, tasks, evaluations,
// SSE streaming, and Next.js static assets — everything except Lark.
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

	if err := serverBootstrap.RunServer(obsConfig); err != nil {
		log.Fatalf("web server exited: %v", err)
	}
}
