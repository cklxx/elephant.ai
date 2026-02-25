// alex-server is the Lark-primary binary.  It runs the Lark WebSocket gateway
// with an embedded lightweight debug HTTP server on :9090 (configurable via
// ALEX_DEBUG_PORT or server.debug_port in config.yaml).
//
// The full web API (auth, sessions, tasks, evaluations, frontend) has moved to
// the alex-web binary (cmd/alex-web).
package main

import (
	"fmt"
	"log"
	"os"

	serverBootstrap "alex/internal/delivery/server/bootstrap"
	runtimeconfig "alex/internal/shared/config"
)

type runners struct {
	runLark         func(string) error
	runKernelDaemon func(string) error
	runKernelOnce   func(string) error
}

func run(args []string, obsConfig string, rs runners) error {
	if rs.runLark == nil {
		rs.runLark = serverBootstrap.RunLark
	}
	if rs.runKernelDaemon == nil {
		rs.runKernelDaemon = serverBootstrap.RunKernelDaemon
	}
	if rs.runKernelOnce == nil {
		rs.runKernelOnce = serverBootstrap.RunKernelOnce
	}

	if len(args) > 1 {
		switch args[1] {
		case "kernel-daemon":
			return rs.runKernelDaemon(obsConfig)
		case "kernel-once":
			return rs.runKernelOnce(obsConfig)
		default:
			return fmt.Errorf("unknown subcommand: %s", args[1])
		}
	}
	return rs.runLark(obsConfig)
}

func main() {
	if err := runtimeconfig.LoadDotEnv(); err != nil {
		log.Printf("Warning: failed to load .env: %v", err)
	}

	obsConfig := os.Getenv("ALEX_OBSERVABILITY_CONFIG")
	if err := run(os.Args, obsConfig, runners{}); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
