package main

import (
	"log"

	agent_eval "alex/evaluation/agent_eval"
)

func main() {
	if err := agent_eval.RunQuickEvaluation(); err != nil {
		log.Fatalf("quick evaluation failed: %v", err)
	}
}
