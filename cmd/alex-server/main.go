package main

import (
	"log"
	"os"

	runtimeconfig "alex/internal/config"
	serverBootstrap "alex/internal/server/bootstrap"
)

func main() {
	if err := runtimeconfig.LoadDotEnv(); err != nil {
		log.Printf("Warning: failed to load .env: %v", err)
	}
	if err := serverBootstrap.RunServer(os.Getenv("ALEX_OBSERVABILITY_CONFIG")); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
