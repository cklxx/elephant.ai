package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const acpProtocolVersion = 1

// handleACP runs the Agent Client Protocol (ACP) server.
func (c *CLI) handleACP(args []string) error {
	if len(args) > 0 && args[0] == "serve" {
		return c.handleACPServe(args[1:])
	}
	return c.handleACPStdio(args)
}

func (c *CLI) handleACPStdio(args []string) error {
	fs := flag.NewFlagSet("acp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	initialMessage := fs.String("initial-message", "", "Seed a system message into new ACP sessions")
	if err := fs.Parse(args); err != nil {
		return err
	}

	server := newACPServer(c.container, strings.TrimSpace(*initialMessage))
	return server.Serve(cliBaseContext(), os.Stdin, os.Stdout)
}

func (c *CLI) handleACPServe(args []string) error {
	fs := flag.NewFlagSet("acp serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	host := fs.String("host", "127.0.0.1", "Host to bind the ACP server")
	port := fs.Int("port", 9000, "Port to bind the ACP server")
	initialMessage := fs.String("initial-message", "", "Seed a system message into new ACP sessions")
	if err := fs.Parse(args); err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", strings.TrimSpace(*host), *port)
	server := newACPServer(c.container, strings.TrimSpace(*initialMessage))
	handler := newACPHTTPServer(server)
	if handler == nil {
		return fmt.Errorf("failed to create ACP HTTP server")
	}

	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler.Handler(),
	}

	fmt.Fprintf(os.Stderr, "ACP SSE server listening on http://%s\n", addr)
	return httpServer.ListenAndServe()
}
