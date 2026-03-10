package main

import (
	"context"
	"fmt"
	"os"
	"time"
	runtimepkg "alex/internal/runtime"
	"alex/internal/runtime/adapter"
	"alex/internal/runtime/panel"
)

func main() {
	paneID := 20
	sd := "/tmp/kaku-debug-test"
	if err := os.MkdirAll(sd, 0755); err != nil {
		fmt.Printf("mkdir: %v\n", err)
		os.Exit(1)
	}

	rt, err := runtimepkg.New(sd, runtimepkg.Config{})
	if err != nil {
		fmt.Printf("new runtime: %v\n", err)
		os.Exit(1)
	}

	pm, err := panel.NewManager()
	if err != nil {
		fmt.Printf("panel manager: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("panel manager created\n")

	fac := adapter.NewFactory(pm, rt, "http://localhost:8080", nil)
	rt.SetFactory(fac)

	s, err := rt.CreateSession("claude_code", "Count .go files: find . -name '*.go' | wc -l. Then stop.", "/Users/bytedance/code/elephant.ai", "")
	if err != nil {
		fmt.Printf("create session: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("session created: %s\n", s.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("calling StartSession with parentPaneID=%d\n", paneID)
	if err := rt.StartSession(ctx, s.ID, paneID); err != nil {
		fmt.Printf("start session ERROR: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("StartSession returned OK\n")
	time.Sleep(2 * time.Second)
}
