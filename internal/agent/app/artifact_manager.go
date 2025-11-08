package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/storage/blobstore"
	"alex/internal/storage/craftsync"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

// ArtifactManager uploads attachments to blob storage and synchronizes session artifact metadata.
type ArtifactManager struct {
	store  blobstore.BlobStore
	logger ports.Logger
	mirror craftsync.Mirror
}

// NewArtifactManager constructs an ArtifactManager using the provided blob store.
func NewArtifactManager(store blobstore.BlobStore, logger ports.Logger, opts ...ArtifactManagerOption) *ArtifactManager {
	if logger == nil {
		logger = utils.NewComponentLogger("ArtifactManager")
	}
	manager := &ArtifactManager{store: store, logger: logger}
	for _, opt := range opts {
		if opt != nil {
			opt(manager)
		}
	}
	return manager
}

// ArtifactManagerOption customises artifact manager behaviour.
type ArtifactManagerOption func(*ArtifactManager)

// WithArtifactMirror configures a mirror for synchronising crafts to the sandbox filesystem.
func WithArtifactMirror(mirror craftsync.Mirror) ArtifactManagerOption {
	return func(manager *ArtifactManager) {
		manager.mirror = mirror
	}
}

// ProcessMessages uploads any inline attachments contained in the message list and appends newly created
// artifact metadata to the provided session.
func (m *ArtifactManager) ProcessMessages(ctx context.Context, session *ports.Session, messages []ports.Message) ([]ports.Message, []ports.Artifact, error) {
	if m == nil || m.store == nil {
		return messages, nil, nil
	}
	if session == nil {
		return messages, nil, errors.New("session is required")
	}
	userID := session.UserID
	if userID == "" {
		userID = id.UserIDFromContext(ctx)
	}
	if userID == "" {
		return messages, nil, errors.New("missing user id for artifact processing")
	}

	existing := make(map[string]struct{})
	for _, artifact := range session.Artifacts {
		if artifact.StorageKey != "" {
			existing[artifact.StorageKey] = struct{}{}
		}
	}

	updated := make([]ports.Message, len(messages))
	var created []ports.Artifact
	now := time.Now()

	for i, message := range messages {
		if len(message.Attachments) == 0 {
			updated[i] = message
			continue
		}
		newAttachments := make(map[string]ports.Attachment, len(message.Attachments))
		for placeholder, attachment := range message.Attachments {
			// Skip if already linked to storage
			if attachment.StorageKey == "" {
				artifact, err := m.persistAttachment(ctx, session, message, attachment, now)
				if err != nil {
					return nil, nil, err
				}
				if artifact != nil {
					attachment.StorageKey = artifact.StorageKey
					attachment.Size = artifact.Size
					attachment.Checksum = artifact.Checksum
					if attachment.Source == "" {
						attachment.Source = artifact.Source
					}
					// Drop the inline payload now that it is persisted to external storage.
					attachment.Data = ""
					if _, seen := existing[artifact.StorageKey]; !seen {
						created = append(created, *artifact)
						existing[artifact.StorageKey] = struct{}{}
					}
				}
			}
			newAttachments[placeholder] = attachment
		}
		message.Attachments = newAttachments
		updated[i] = message
	}

	if len(created) > 0 {
		session.Artifacts = append(session.Artifacts, created...)
	}

	return updated, created, nil
}

func (m *ArtifactManager) persistAttachment(ctx context.Context, session *ports.Session, message ports.Message, attachment ports.Attachment, timestamp time.Time) (*ports.Artifact, error) {
	messageID := extractMessageID(message)

	if attachment.Data == "" {
		// No inline payload; nothing to persist but still return metadata entry if we have a URI.
		if attachment.URI == "" {
			return nil, nil
		}
		artifact := ports.Artifact{
			ID:          id.NewArtifactID(),
			SessionID:   session.ID,
			UserID:      session.UserID,
			MessageID:   messageID,
			Name:        attachment.Name,
			MediaType:   attachment.MediaType,
			URI:         attachment.URI,
			Description: attachment.Description,
			Source:      defaultSource(attachment.Source),
			CreatedAt:   timestamp,
		}
		m.mirrorArtifact(ctx, artifact, nil)
		return &artifact, nil
	}

	decoded, err := decodeBase64Data(attachment.Data)
	if err != nil {
		return nil, fmt.Errorf("decode attachment %s: %w", attachment.Name, err)
	}

	artifactID := id.NewArtifactID()
	storageKey := fmt.Sprintf("%s/%s", session.UserID, artifactID)
	reader := bytes.NewReader(decoded)
	_, err = m.store.PutObject(ctx, storageKey, reader, blobstore.PutOptions{ContentType: attachment.MediaType, ContentLength: int64(len(decoded))})
	if err != nil {
		return nil, fmt.Errorf("upload attachment %s: %w", attachment.Name, err)
	}

	checksum := checksum(decoded)
	artifact := ports.Artifact{
		ID:          artifactID,
		SessionID:   session.ID,
		UserID:      session.UserID,
		MessageID:   messageID,
		Name:        attachment.Name,
		MediaType:   attachment.MediaType,
		StorageKey:  storageKey,
		Description: attachment.Description,
		Size:        int64(len(decoded)),
		Checksum:    checksum,
		Source:      defaultSource(attachment.Source),
		CreatedAt:   timestamp,
	}
	m.mirrorArtifact(ctx, artifact, decoded)
	return &artifact, nil
}

func decodeBase64Data(data string) ([]byte, error) {
	payload := data
	if idx := strings.Index(data, ","); idx != -1 {
		payload = data[idx+1:]
	}
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, errors.New("empty attachment payload")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		// Try URL-safe encoding as fallback
		decoded, err = base64.URLEncoding.DecodeString(payload)
		if err != nil {
			return nil, err
		}
	}
	return decoded, nil
}

func checksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func defaultSource(source string) string {
	if strings.TrimSpace(source) != "" {
		return source
	}
	return "generated"
}

func extractMessageID(msg ports.Message) string {
	if msg.Metadata == nil {
		return ""
	}
	if value, ok := msg.Metadata["message_id"]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

func (m *ArtifactManager) mirrorArtifact(ctx context.Context, artifact ports.Artifact, content []byte) {
	if m == nil || m.mirror == nil {
		return
	}
	meta := craftsync.ArtifactMetadata{
		ID:          artifact.ID,
		UserID:      artifact.UserID,
		SessionID:   artifact.SessionID,
		Name:        artifact.Name,
		MediaType:   artifact.MediaType,
		Description: artifact.Description,
		Source:      artifact.Source,
		StorageKey:  artifact.StorageKey,
		URI:         artifact.URI,
		Size:        artifact.Size,
		Checksum:    artifact.Checksum,
		CreatedAt:   artifact.CreatedAt,
	}
	if _, err := m.mirror.Mirror(ctx, meta, content); err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to mirror craft %s: %v", artifact.ID, err)
		}
	}
}
