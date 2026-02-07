package main

import (
	"flag"
	"log"
	"os"

	"alex/internal/delivery/eval/bootstrap"
)

func main() {
	configPath := flag.String("config", "", "path to eval-server config YAML")
	flag.Parse()

	if err := bootstrap.RunEvalServer(*configPath); err != nil {
		log.Printf("[eval-server] fatal: %v", err)
		os.Exit(1)
	}
}
