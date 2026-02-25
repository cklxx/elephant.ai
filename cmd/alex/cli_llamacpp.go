package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"alex/internal/infra/llamacpp"
	runtimeconfig "alex/internal/shared/config"
)

func (c *CLI) handleLlamaCpp(args []string) error {
	return executeLlamaCppCommand(args, os.Stdout, runtimeEnvLookup())
}

func executeLlamaCppCommand(args []string, out io.Writer, envLookup runtimeconfig.EnvLookup) error {
	subcommand := ""
	if len(args) > 0 {
		subcommand = strings.ToLower(strings.TrimSpace(args[0]))
	}
	switch subcommand {
	case "", "help", "-h", "--help":
		printLlamaCppUsage(out)
		return nil
	case "pull", "download":
		return pullLlamaCppWeights(out, args[1:], envLookup)
	default:
		printLlamaCppUsage(out)
		return fmt.Errorf("unknown llama-cpp subcommand: %s", subcommand)
	}
}

func pullLlamaCppWeights(out io.Writer, args []string, envLookup runtimeconfig.EnvLookup) error {
	fs := flag.NewFlagSet("alex llama-cpp pull", flag.ContinueOnError)
	fs.SetOutput(out)

	var revision string
	var dir string
	var sha256 string
	var token string
	var hfBaseURL string
	var timeoutSeconds int

	fs.StringVar(&revision, "revision", "main", "Hugging Face revision (branch/tag/commit)")
	fs.StringVar(&dir, "dir", "", "Model cache root dir (default ~/.alex/models/llama.cpp)")
	fs.StringVar(&sha256, "sha256", "", "Expected SHA256 hex digest (optional)")
	fs.StringVar(&token, "token", "", "Hugging Face token (optional; defaults to HUGGINGFACE_TOKEN env)")
	fs.StringVar(&hfBaseURL, "hf-base-url", "", "Hugging Face base URL (testing only)")
	fs.IntVar(&timeoutSeconds, "timeout-seconds", 0, "HTTP timeout seconds (optional)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	pos := fs.Args()
	if len(pos) < 2 || strings.TrimSpace(pos[0]) == "" || strings.TrimSpace(pos[1]) == "" {
		return fmt.Errorf("usage: alex llama-cpp pull <hf_repo> <gguf_file> [--revision main] [--dir ~/.alex/models/llama.cpp]")
	}

	if token == "" && envLookup != nil {
		if value, ok := envLookup("HUGGINGFACE_TOKEN"); ok {
			token = strings.TrimSpace(value)
		}
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	ctx, cancel := context.WithCancel(cliBaseContext())
	defer cancel()

	path, err := llamacpp.DownloadGGUF(ctx, llamacpp.GGUFRef{
		Repo:     pos[0],
		File:     pos[1],
		Revision: revision,
		SHA256:   sha256,
	}, llamacpp.DownloadOptions{
		BaseDir:   dir,
		HFBaseURL: hfBaseURL,
		HFToken:   token,
		Timeout:   timeout,
	})
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(out, "Downloaded: %s\n", path); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Start llama-server (example): llama-server -m %q --port 8082\n", path); err != nil {
		return err
	}
	return nil
}

func printLlamaCppUsage(out io.Writer) {
	lines := []string{
		"Llama.cpp command usage:",
		"  alex llama-cpp pull <hf_repo> <gguf_file> [flags]   Download GGUF weights from Hugging Face",
		"",
		"Flags:",
		"  --revision main                 Hugging Face revision (branch/tag/commit)",
		"  --dir ~/.alex/models/llama.cpp  Destination root dir",
		"  --sha256 <hex>                  Optional integrity check",
		"  --token <token>                 Optional; defaults to HUGGINGFACE_TOKEN env",
		"",
		"Examples:",
		"  alex llama-cpp pull TheBloke/Meta-Llama-3-8B-Instruct-GGUF meta-llama-3-8b-instruct-q4_k_m.gguf",
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(out, line); err != nil {
			return
		}
	}
}
