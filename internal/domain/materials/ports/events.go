package ports

import (
	"context"

	materialapi "alex/internal/domain/materials/api"
)

// EventPublisher publishes newly registered materials onto an external event bus.
type EventPublisher interface {
	PublishMaterial(ctx context.Context, material *materialapi.Material) error
}
