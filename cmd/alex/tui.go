package main

import (
	"fmt"
	"os"

	"alex/internal/delivery/output"
)

// RunNativeChatUI starts the interactive chat UI using native line-mode.
func RunNativeChatUI(container *Container) error {
	if container == nil {
		return fmt.Errorf("container is nil")
	}

	output.ConfigureCLIColorProfile(os.Stdout)
	return runLineChatUI(container, os.Stdin, os.Stdout, os.Stderr)
}
