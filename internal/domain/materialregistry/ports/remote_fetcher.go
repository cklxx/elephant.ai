package ports

import "context"

// RemoteFetcher fetches remote resources by URI.
type RemoteFetcher interface {
	Fetch(ctx context.Context, uri string, expectedMediaType string) (data []byte, contentType string, err error)
}
