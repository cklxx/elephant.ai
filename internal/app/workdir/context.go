package workdir

import (
	"context"

	"alex/internal/infra/tools/builtin/pathutil"
)

// DefaultWorkingDir returns the current process working directory.
func DefaultWorkingDir() string {
	return pathutil.DefaultWorkingDir()
}

// WithWorkingDir stores a working directory hint in context.
func WithWorkingDir(ctx context.Context, workingDir string) context.Context {
	return pathutil.WithWorkingDir(ctx, workingDir)
}
