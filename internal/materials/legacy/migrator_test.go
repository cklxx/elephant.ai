package legacy

import (
	"context"
	"encoding/base64"
	"testing"

	"alex/internal/agent/ports"
	materialapi "alex/internal/materials/api"
	"alex/internal/materials/broker"
	materialports "alex/internal/materials/ports"
	"alex/internal/materials/storage"
)

type stubRegistry struct{}

func (stubRegistry) RegisterMaterials(ctx context.Context, req *materialapi.RegisterMaterialsRequest) (*materialapi.RegisterMaterialsResponse, error) {
	materials := make([]*materialapi.Material, 0, len(req.Materials))
	for _, input := range req.Materials {
		materials = append(materials, &materialapi.Material{
			MaterialID: "mat-" + input.Name,
			Descriptor: &materialapi.MaterialDescriptor{
				Name:        input.Name,
				Placeholder: "[material:" + input.Name + "]",
				MimeType:    input.MimeType,
				Tags:        map[string]string{"placeholder": input.Tags["placeholder"]},
			},
			Storage: &materialapi.MaterialStorage{CDNURL: input.CDNURL, StorageKey: input.StorageKey},
		})
	}
	return &materialapi.RegisterMaterialsResponse{Materials: materials}, nil
}

func (stubRegistry) ListMaterials(ctx context.Context, req *materialapi.ListMaterialsRequest) (*materialapi.ListMaterialsResponse, error) {
	return &materialapi.ListMaterialsResponse{}, nil
}

func TestBrokerMigratorUploadsInlinePayloads(t *testing.T) {
	mapper := storage.NewInMemoryMapper("https://cdn")
	reg := stubRegistry{}
	b, err := broker.NewAttachmentBroker(reg, mapper)
	if err != nil {
		t.Fatalf("new broker: %v", err)
	}
	migrator := NewBrokerMigrator(b)
	attachments := map[string]ports.Attachment{
		"[foo.png]": {Name: "foo.png", MediaType: "image/png", Data: base64.StdEncoding.EncodeToString([]byte("bytes"))},
	}
	ctx := context.Background()
	result, err := migrator.Normalize(ctx, materialports.MigrationRequest{
		Context:     &materialapi.RequestContext{RequestID: "req"},
		Attachments: attachments,
		Status:      materialapi.MaterialStatusIntermediate,
	})
	if err != nil {
		t.Fatalf("normalize returned error: %v", err)
	}
	att := result["[foo.png]"]
	if att.URI == "" {
		t.Fatalf("expected CDN URI after migration, got %+v", att)
	}
	if _, ok := result["[material:foo.png]"]; !ok {
		t.Fatalf("expected material placeholder to exist in normalized map")
	}
}

func TestBrokerMigratorSkipsHTTPAttachments(t *testing.T) {
	migrator := NewBrokerMigrator(nil)
	attachments := map[string]ports.Attachment{
		"[existing.png]": {Name: "existing.png", MediaType: "image/png", URI: "https://cdn/existing.png"},
	}
	result, err := migrator.Normalize(context.Background(), materialports.MigrationRequest{Attachments: attachments})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["[existing.png]"].URI != "https://cdn/existing.png" {
		t.Fatalf("expected URI to remain unchanged")
	}
}
