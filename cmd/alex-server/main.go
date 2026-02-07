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

	if len(os.Args) > 1 && os.Args[1] == "lark" {
		if err := serverBootstrap.RunLark(obsConfig); err != nil {
			log.Fatalf("lark mode exited: %v", err)
		}
		return
	}

	if err := serverBootstrap.RunServer(obsConfig); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
