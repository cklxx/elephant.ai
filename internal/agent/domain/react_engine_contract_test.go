package domain

import "alex/internal/agent/ports"

var _ ports.ReactiveExecutor = (*ReactEngine)(nil)
