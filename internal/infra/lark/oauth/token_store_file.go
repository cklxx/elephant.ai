package oauth

import (
	"context"
	"fmt"
	"path/filepath"

	"alex/internal/infra/filestore"
	jsonx "alex/internal/shared/json"
)

type FileTokenStore struct {
	coll *filestore.Collection[string, Token]
}

func NewFileTokenStore(dir string) (*FileTokenStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("dir required")
	}
	if err := filestore.EnsureDir(dir); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	coll := filestore.NewCollection[string, Token](filestore.CollectionConfig{
		FilePath: filepath.Join(dir, "tokens.json"),
		Perm:     0o600,
		Name:     "oauth_token",
	})
	// Direct map format (no envelope wrapper).
	coll.SetUnmarshalDoc(func(data []byte) (map[string]Token, error) {
		var decoded map[string]Token
		if err := jsonx.Unmarshal(data, &decoded); err != nil {
			return nil, err
		}
		m := make(map[string]Token, len(decoded))
		for k, v := range decoded {
			if k == "" || v.OpenID == "" {
				continue
			}
			m[k] = v
		}
		return m, nil
	})
	_ = coll.Load()
	return &FileTokenStore{coll: coll}, nil
}

func (s *FileTokenStore) EnsureSchema(_ context.Context) error { return nil }

func (s *FileTokenStore) Get(ctx context.Context, openID string) (Token, error) {
	if ctx != nil && ctx.Err() != nil {
		return Token{}, ctx.Err()
	}
	if openID == "" {
		return Token{}, fmt.Errorf("open_id required")
	}
	token, ok := s.coll.Get(openID)
	if !ok {
		return Token{}, ErrTokenNotFound
	}
	return token, nil
}

func (s *FileTokenStore) Upsert(ctx context.Context, token Token) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if token.OpenID == "" {
		return fmt.Errorf("open_id required")
	}
	if token.UpdatedAt.IsZero() {
		token.UpdatedAt = s.coll.Now()
	}
	return s.coll.Put(token.OpenID, token)
}

func (s *FileTokenStore) Delete(ctx context.Context, openID string) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if openID == "" {
		return fmt.Errorf("open_id required")
	}
	return s.coll.Delete(openID)
}

var _ TokenStore = (*FileTokenStore)(nil)
