package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/storage/blobstore"
	"alex/internal/storage/craftsync"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

// Craft represents a persisted artifact exposed to API consumers.
type Craft struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	MediaType   string    `json:"media_type"`
	Description string    `json:"description,omitempty"`
	Source      string    `json:"source,omitempty"`
	Size        int64     `json:"size,omitempty"`
	Checksum    string    `json:"checksum,omitempty"`
	StorageKey  string    `json:"storage_key"`
	CreatedAt   time.Time `json:"created_at"`
}

// CraftService aggregates artifacts from sessions and mediates blob store operations.
type CraftService struct {
	sessionStore ports.SessionStore
	blobStore    blobstore.BlobStore
	mirror       craftsync.Mirror
	logger       *utils.Logger
}

// NewCraftService constructs a CraftService instance.
func NewCraftService(sessionStore ports.SessionStore, blobStore blobstore.BlobStore, mirror craftsync.Mirror) *CraftService {
	return &CraftService{
		sessionStore: sessionStore,
		blobStore:    blobStore,
		mirror:       mirror,
		logger:       utils.NewComponentLogger("CraftService"),
	}
}

// List returns all crafts belonging to the user present in context.
func (s *CraftService) List(ctx context.Context) ([]Craft, error) {
	if s == nil {
		return nil, errors.New("craft service not initialized")
	}
	sessionIDs, err := s.sessionStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	crafts := make([]Craft, 0)
	userID := id.UserIDFromContext(ctx)
	for _, sessionID := range sessionIDs {
		session, err := s.sessionStore.Get(ctx, sessionID)
		if err != nil {
			continue
		}
		if userID != "" {
			if session.UserID == "" || session.UserID != userID {
				continue
			}
		}
		for _, artifact := range session.Artifacts {
			crafts = append(crafts, Craft{
				ID:          artifact.ID,
				SessionID:   artifact.SessionID,
				UserID:      artifact.UserID,
				Name:        artifact.Name,
				MediaType:   artifact.MediaType,
				Description: artifact.Description,
				Source:      artifact.Source,
				Size:        artifact.Size,
				Checksum:    artifact.Checksum,
				StorageKey:  artifact.StorageKey,
				CreatedAt:   artifact.CreatedAt,
			})
		}
	}
	sort.Slice(crafts, func(i, j int) bool {
		return crafts[i].CreatedAt.After(crafts[j].CreatedAt)
	})
	return crafts, nil
}

// Delete removes a craft and deletes the backing object from blob storage.
func (s *CraftService) Delete(ctx context.Context, craftID string) error {
	session, artifact, err := s.findArtifact(ctx, craftID)
	if err != nil {
		return err
	}
	if artifact.StorageKey != "" && s.blobStore != nil {
		if err := s.blobStore.DeleteObject(ctx, artifact.StorageKey); err != nil {
			s.logger.Warn("Failed to delete blob %s: %v", artifact.StorageKey, err)
		}
	}

	filtered := session.Artifacts[:0]
	for _, item := range session.Artifacts {
		if item.ID == craftID {
			continue
		}
		filtered = append(filtered, item)
	}
	session.Artifacts = filtered
	if err := s.sessionStore.Save(ctx, session); err != nil {
		return err
	}

	s.removeMirror(ctx, *artifact)
	return nil
}

// DownloadURL generates a signed URL for the craft if possible.
func (s *CraftService) DownloadURL(ctx context.Context, craftID string, expiry time.Duration) (string, error) {
	_, artifact, err := s.findArtifact(ctx, craftID)
	if err != nil {
		return "", err
	}
	if s.blobStore == nil || artifact.StorageKey == "" {
		return "", errors.New("artifact is not stored in blob store")
	}
	if expiry <= 0 {
		expiry = 10 * time.Minute
	}
	url, err := s.blobStore.GetSignedURL(ctx, artifact.StorageKey, expiry)
	if err != nil {
		return "", fmt.Errorf("generate signed url: %w", err)
	}
	return url, nil
}

func (s *CraftService) findArtifact(ctx context.Context, craftID string) (*ports.Session, *ports.Artifact, error) {
	sessionIDs, err := s.sessionStore.List(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list sessions: %w", err)
	}
	userID := id.UserIDFromContext(ctx)
	for _, sessionID := range sessionIDs {
		session, err := s.sessionStore.Get(ctx, sessionID)
		if err != nil {
			continue
		}
		if userID != "" && session.UserID != "" && session.UserID != userID {
			continue
		}
		for _, artifact := range session.Artifacts {
			if artifact.ID == craftID {
				return session, &artifact, nil
			}
		}
	}
	return nil, nil, errors.New("craft not found")
}

func (s *CraftService) removeMirror(ctx context.Context, artifact ports.Artifact) {
	if s == nil || s.mirror == nil {
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
	if err := s.mirror.Remove(ctx, meta); err != nil {
		if s.logger != nil {
			s.logger.Warn("failed to remove mirrored craft %s: %v", artifact.ID, err)
		}
	}
}
