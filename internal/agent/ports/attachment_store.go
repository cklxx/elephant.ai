package ports

import "context"

// AttachmentPersister persists inline attachment payloads (base64 Data or
// data: URIs) to a durable store and returns the updated attachment with a
// stable, fetchable URI.  Implementations live in the infrastructure layer
// (e.g. internal/attachments) while the domain layer depends only on this
// interface.
type AttachmentPersister interface {
	// Persist writes the inline payload to durable storage and returns a
	// copy of the attachment with URI populated and Data cleared. Callers
	// should pass a context with timeout/cancellation.
	//
	// Behaviour:
	//   - If the attachment already has an external URI and no inline data,
	//     it is returned unchanged.
	//   - Small text-like payloads (text/*, markdown, json) below the
	//     implementation's retention limit may keep Data populated for
	//     frontend preview convenience.
	//   - On error the original attachment is returned alongside the error
	//     so callers can degrade gracefully.
	Persist(ctx context.Context, att Attachment) (Attachment, error)
}
