//go:build cgo
// +build cgo

package memory

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// StoredChunk represents a chunk stored in the index.
type StoredChunk struct {
	ID        int64
	Path      string
	StartLine int
	EndLine   int
	Text      string
}

// VectorMatch captures a vector search match.
type VectorMatch struct {
	Chunk    StoredChunk
	Distance float64
}

// TextMatch captures a BM25 search match.
type TextMatch struct {
	Chunk StoredChunk
	BM25  float64
}

// IndexedChunk captures a chunk ready for insertion.
type IndexedChunk struct {
	Path      string
	StartLine int
	EndLine   int
	Text      string
	Hash      string
	Embedding []float32
}

// IndexStore persists memory chunks and vectors in SQLite.
type IndexStore struct {
	path string
	db   *sql.DB
	// ftsEnabled indicates whether FTS5 is available.
	ftsEnabled bool
}

// OpenIndexStore opens (or creates) a SQLite-backed index store.
func OpenIndexStore(path string) (*IndexStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("index path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	ensureSQLiteVecDriverRegistered()
	db, err := sql.Open(sqliteVecDriverName, path)
	if err != nil {
		return nil, err
	}
	return &IndexStore{path: path, db: db}, nil
}

// EnsureSchema creates tables if missing. Provide dim for vec0 column creation.
func (s *IndexStore) EnsureSchema(ctx context.Context, dim int) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("index store not initialized")
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys=ON;`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS chunks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	path TEXT NOT NULL,
	start_line INTEGER NOT NULL,
	end_line INTEGER NOT NULL,
	text TEXT NOT NULL,
	hash TEXT NOT NULL
);`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts
USING fts5(text, content='');`); err != nil {
		if isMissingModule(err) {
			s.ftsEnabled = false
		} else {
			return err
		}
	} else {
		s.ftsEnabled = true
	}
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS embedding_cache (
	hash TEXT PRIMARY KEY,
	embedding BLOB NOT NULL
);`); err != nil {
		return err
	}
	if dim > 0 {
		stmt := fmt.Sprintf(
			`CREATE VIRTUAL TABLE IF NOT EXISTS chunks_vec USING vec0(embedding float[%d]);`,
			dim,
		)
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// Close releases database resources.
func (s *IndexStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// LookupEmbeddings returns cached embeddings for the given hashes.
func (s *IndexStore) LookupEmbeddings(ctx context.Context, hashes []string) (map[string][]float32, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("index store not initialized")
	}
	if len(hashes) == 0 {
		return map[string][]float32{}, nil
	}
	placeholders := strings.Repeat("?,", len(hashes))
	placeholders = strings.TrimSuffix(placeholders, ",")
	query := fmt.Sprintf(`SELECT hash, embedding FROM embedding_cache WHERE hash IN (%s);`, placeholders)
	args := make([]any, 0, len(hashes))
	for _, hash := range hashes {
		args = append(args, hash)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]float32, len(hashes))
	for rows.Next() {
		var hash string
		var blob []byte
		if err := rows.Scan(&hash, &blob); err != nil {
			return nil, err
		}
		out[hash] = bytesToFloat32s(blob)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ReplaceChunks replaces all chunks for a path with the provided chunks.
func (s *IndexStore) ReplaceChunks(ctx context.Context, path string, chunks []IndexedChunk) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("index store not initialized")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := deleteByPathTx(ctx, tx, path); err != nil {
		_ = tx.Rollback()
		return err
	}

	insertChunk := `INSERT INTO chunks(path, start_line, end_line, text, hash) VALUES (?, ?, ?, ?, ?);`
	insertVec := `INSERT INTO chunks_vec(rowid, embedding) VALUES (?, vec_f32(?));`
	insertFTS := `INSERT INTO chunks_fts(rowid, text) VALUES (?, ?);`
	insertCache := `INSERT OR IGNORE INTO embedding_cache(hash, embedding) VALUES (?, ?);`

	for _, chunk := range chunks {
		res, err := tx.ExecContext(ctx, insertChunk, path, chunk.StartLine, chunk.EndLine, chunk.Text, chunk.Hash)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		embeddingBytes := float32sToBytes(chunk.Embedding)
		if _, err := tx.ExecContext(ctx, insertVec, id, embeddingBytes); err != nil {
			_ = tx.Rollback()
			return err
		}
		if s.ftsEnabled {
			if _, err := tx.ExecContext(ctx, insertFTS, id, chunk.Text); err != nil && !isMissingTable(err) {
				_ = tx.Rollback()
				return err
			}
		}
		if chunk.Hash != "" && len(embeddingBytes) > 0 {
			if _, err := tx.ExecContext(ctx, insertCache, chunk.Hash, embeddingBytes); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
	}

	return tx.Commit()
}

// DeleteByPath removes all chunks associated with the path.
func (s *IndexStore) DeleteByPath(ctx context.Context, path string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("index store not initialized")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM chunks_vec WHERE rowid IN (SELECT id FROM chunks WHERE path = ?);`, path); err != nil && !isMissingTable(err) {
		return err
	}
	if s.ftsEnabled {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM chunks_fts WHERE rowid IN (SELECT id FROM chunks WHERE path = ?);`, path); err != nil && !isMissingTable(err) {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM chunks WHERE path = ?;`, path); err != nil && !isMissingTable(err) {
		return err
	}
	return nil
}

// SearchVector performs vector similarity search.
func (s *IndexStore) SearchVector(ctx context.Context, embedding []float32, limit int) ([]VectorMatch, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("index store not initialized")
	}
	if len(embedding) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultSearchMax
	}
	query := `
SELECT c.id, c.path, c.start_line, c.end_line, c.text, v.distance
FROM chunks_vec v
JOIN chunks c ON c.id = v.rowid
WHERE v.embedding MATCH vec_f32(?) AND v.k = ?;`
	rows, err := s.db.QueryContext(ctx, query, float32sToBytes(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []VectorMatch
	for rows.Next() {
		var chunk StoredChunk
		var distance float64
		if err := rows.Scan(&chunk.ID, &chunk.Path, &chunk.StartLine, &chunk.EndLine, &chunk.Text, &distance); err != nil {
			return nil, err
		}
		matches = append(matches, VectorMatch{Chunk: chunk, Distance: distance})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return matches, nil
}

// SearchBM25 performs full-text search using FTS5 BM25 ranking.
func (s *IndexStore) SearchBM25(ctx context.Context, queryText string, limit int) ([]TextMatch, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("index store not initialized")
	}
	if !s.ftsEnabled {
		return nil, nil
	}
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultSearchMax
	}
	query := `
SELECT c.id, c.path, c.start_line, c.end_line, c.text, bm25(chunks_fts) AS score
FROM chunks_fts
JOIN chunks c ON c.id = chunks_fts.rowid
WHERE chunks_fts MATCH ?
ORDER BY score
LIMIT ?;`
	rows, err := s.db.QueryContext(ctx, query, queryText, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []TextMatch
	for rows.Next() {
		var chunk StoredChunk
		var score float64
		if err := rows.Scan(&chunk.ID, &chunk.Path, &chunk.StartLine, &chunk.EndLine, &chunk.Text, &score); err != nil {
			return nil, err
		}
		matches = append(matches, TextMatch{Chunk: chunk, BM25: score})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return matches, nil
}

func deleteByPathTx(ctx context.Context, tx *sql.Tx, path string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks_vec WHERE rowid IN (SELECT id FROM chunks WHERE path = ?);`, path); err != nil && !isMissingTable(err) {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks_fts WHERE rowid IN (SELECT id FROM chunks WHERE path = ?);`, path); err != nil && !isMissingTable(err) {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE path = ?;`, path); err != nil && !isMissingTable(err) {
		return err
	}
	return nil
}

func isMissingTable(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no such table")
}

func isMissingModule(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no such module: fts5")
}

func float32sToBytes(values []float32) []byte {
	if len(values) == 0 {
		return nil
	}
	buf := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

func bytesToFloat32s(data []byte) []float32 {
	if len(data) == 0 {
		return nil
	}
	count := len(data) / 4
	out := make([]float32, count)
	for i := 0; i < count; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return out
}
