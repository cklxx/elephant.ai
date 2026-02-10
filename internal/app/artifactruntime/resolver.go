package artifactruntime

import (
	"context"
	"net/http"
	"time"

	artifacts "alex/internal/infra/tools/builtin/artifacts"
)

const AttachmentFetchTimeout = artifacts.AttachmentFetchTimeout

// NewAttachmentHTTPClient creates the transport used for attachment fetches.
func NewAttachmentHTTPClient(timeout time.Duration, loggerName string) *http.Client {
	return artifacts.NewAttachmentHTTPClient(timeout, loggerName)
}

// ResolveAttachmentBytes resolves an attachment reference from context or URL.
func ResolveAttachmentBytes(ctx context.Context, ref string, client *http.Client) ([]byte, string, error) {
	return artifacts.ResolveAttachmentBytes(ctx, ref, client)
}
