package main

import (
	"log"
	"os"

	serverBootstrap "alex/internal/server/bootstrap"
)

func main() {
	if err := serverBootstrap.RunServer(os.Getenv("ALEX_OBSERVABILITY_CONFIG")); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
