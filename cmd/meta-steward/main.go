package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"alex/internal/meta"
)

func main() {
	input := flag.String("input", "", "Directory containing journal JSONL files")
	output := flag.String("output", "", "Path to write stewarded meta context (JSON)")
	persona := flag.String("persona", "", "Persona ID to bind the meta profile to")
	version := flag.String("version", "", "Persona version to stamp into the meta profile")
	flag.Parse()

	steward := meta.NewSteward()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	metaCtx, err := steward.Run(ctx, meta.ReplayConfig{
		InputDir:       *input,
		OutputPath:     *output,
		PersonaID:      *persona,
		PersonaVersion: *version,
	})
	if err != nil {
		log.Fatalf("steward failed: %v", err)
	}
	if err := meta.ValidateOutput(metaCtx); err != nil {
		log.Fatalf("stewarded meta invalid: %v", err)
	}
	fmt.Printf("stewarded %d memories and %d recommendations\n", len(metaCtx.Memories), len(metaCtx.Recommendations))
}
