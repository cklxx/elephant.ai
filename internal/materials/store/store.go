package store

import (
	"context"
	"time"

	materialapi "alex/internal/materials/api"
)

// LineageRecord represents a parent relationship stored alongside a material.
type LineageRecord struct {
	ParentMaterialID string
	DerivationType   string
	ParametersHash   string
}

// MaterialRecord is the persistence model consumed by concrete stores.
type MaterialRecord struct {
	MaterialID       string
	Context          *materialapi.RequestContext
	Descriptor       *materialapi.MaterialDescriptor
	Storage          *materialapi.MaterialStorage
	SystemAttributes *materialapi.SystemAttributes
	AccessBindings   []*materialapi.AccessBinding
	Lineage          []LineageRecord
}

// DeleteExpiredMaterialsRequest configures a cleanup sweep.
type DeleteExpiredMaterialsRequest struct {
	Statuses []materialapi.MaterialStatus
	Limit    int
	Now      time.Time
}

// DeletedMaterial contains metadata for downstream cleanup (events, storage).
type DeletedMaterial struct {
	MaterialID       string
	RequestID        string
	StorageKey       string
	PreviewAssetKeys []string
}

// Store persists registered materials plus their lineage metadata.
type Store interface {
	InsertMaterials(ctx context.Context, materials []MaterialRecord) error
	DeleteExpiredMaterials(ctx context.Context, req DeleteExpiredMaterialsRequest) ([]DeletedMaterial, error)
	UpdateRetention(ctx context.Context, materialID string, ttlSeconds uint64) error
}
