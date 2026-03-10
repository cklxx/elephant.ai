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
	"sort"
	"strings"

	"alex/internal/shared/utils"
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
	Edges     []MemoryEdge
}

// MemoryEdge represents a graph edge between memory chunks/files.
type MemoryEdge struct {
	DstPath   string
	DstAnchor string
	EdgeType  string
	Direction string
}

// RelatedMatch captures a linked memory entry returned by graph traversal.
type RelatedMatch struct {
	Path      string
	Anchor    string
	EdgeType  string
	StartLine int
	EndLine   int
	Text      string
	Score     float64
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
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS memory_edges (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	src_path TEXT NOT NULL,
	src_start_line INTEGER NOT NULL,
	src_end_line INTEGER NOT NULL,
	dst_path TEXT NOT NULL,
	dst_anchor TEXT NOT NULL DEFAULT '',
	edge_type TEXT NOT NULL DEFAULT 'related',
	direction TEXT NOT NULL DEFAULT 'directed',
	UNIQUE(src_path, src_start_line, src_end_line, dst_path, dst_anchor, edge_type, direction)
);`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_memory_edges_src ON memory_edges(src_path, src_start_line, src_end_line);`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_memory_edges_dst ON memory_edges(dst_path);`); err != nil {
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
	defer tx.Rollback() //nolint:errcheck // rollback is no-op after commit

	if err := deleteBySourcePathTx(ctx, tx, path); err != nil {
		return err
	}

	insertChunk := `INSERT INTO chunks(path, start_line, end_line, text, hash) VALUES (?, ?, ?, ?, ?);`
	insertVec := `INSERT INTO chunks_vec(rowid, embedding) VALUES (?, vec_f32(?));`
	insertFTS := `INSERT INTO chunks_fts(rowid, text) VALUES (?, ?);`
	insertCache := `INSERT OR IGNORE INTO embedding_cache(hash, embedding) VALUES (?, ?);`
	insertEdge := `INSERT OR IGNORE INTO memory_edges(src_path, src_start_line, src_end_line, dst_path, dst_anchor, edge_type, direction) VALUES (?, ?, ?, ?, ?, ?, ?);`

	for _, chunk := range chunks {
		res, err := tx.ExecContext(ctx, insertChunk, path, chunk.StartLine, chunk.EndLine, chunk.Text, chunk.Hash)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		embeddingBytes := float32sToBytes(chunk.Embedding)
		if _, err := tx.ExecContext(ctx, insertVec, id, embeddingBytes); err != nil {
			return err
		}
		if s.ftsEnabled {
			if _, err := tx.ExecContext(ctx, insertFTS, id, chunk.Text); err != nil && !isMissingTable(err) {
				return err
			}
		}
		if chunk.Hash != "" && len(embeddingBytes) > 0 {
			if _, err := tx.ExecContext(ctx, insertCache, chunk.Hash, embeddingBytes); err != nil {
				return err
			}
		}
		for _, edge := range chunk.Edges {
			dstPath := strings.TrimSpace(edge.DstPath)
			if dstPath == "" {
				continue
			}
			edgeType := strings.TrimSpace(edge.EdgeType)
			if edgeType == "" {
				edgeType = "related"
			}
			direction := strings.TrimSpace(edge.Direction)
			if direction == "" {
				direction = "directed"
			}
			if _, err := tx.ExecContext(
				ctx,
				insertEdge,
				path,
				chunk.StartLine,
				chunk.EndLine,
				dstPath,
				strings.TrimSpace(edge.DstAnchor),
				edgeType,
				direction,
			); err != nil {
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback is no-op after commit
	if err := deleteByPathTx(ctx, tx, path); err != nil {
		return err
	}
	return tx.Commit()
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

// CountRelated returns how many graph edges are connected to the source chunk.
func (s *IndexStore) CountRelated(ctx context.Context, path string, startLine, endLine int) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("index store not initialized")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return 0, nil
	}
	if startLine <= 0 || endLine <= 0 {
		var count int
		if err := s.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM memory_edges WHERE src_path = ? OR dst_path = ?;`,
			path,
			path,
		).Scan(&count); err != nil {
			return 0, err
		}
		return count, nil
	}
	var count int
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM memory_edges
WHERE (src_path = ? AND src_start_line = ? AND src_end_line = ?)
   OR (dst_path = ?);`,
		path, startLine, endLine, path,
	).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// SearchRelated returns graph-adjacent memory entries for a source path/range.
func (s *IndexStore) SearchRelated(ctx context.Context, path string, fromLine, toLine, limit int) ([]RelatedMatch, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("index store not initialized")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if limit <= 0 {
		limit = defaultSearchMax
	}

	outgoing, err := s.fetchRelatedRows(ctx, `
SELECT
	e.dst_path,
	e.dst_anchor,
	e.edge_type,
	COALESCE(MIN(c.start_line), 1) AS start_line,
	COALESCE(MIN(c.end_line), 1) AS end_line,
	COALESCE(MIN(c.text), '') AS text,
	1.0 AS score
FROM memory_edges e
LEFT JOIN chunks c ON c.path = e.dst_path
WHERE e.src_path = ?
  AND (
	(? <= 0 OR ? <= 0)
	OR (e.src_end_line >= ? AND e.src_start_line <= ?)
  )
GROUP BY e.dst_path, e.dst_anchor, e.edge_type
LIMIT ?;`, path, fromLine, toLine, fromLine, toLine, limit)
	if err != nil {
		return nil, err
	}
	incoming, err := s.fetchRelatedRows(ctx, `
SELECT
	e.src_path AS dst_path,
	'' AS dst_anchor,
	e.edge_type,
	COALESCE(MIN(c.start_line), 1) AS start_line,
	COALESCE(MIN(c.end_line), 1) AS end_line,
	COALESCE(MIN(c.text), '') AS text,
	0.8 AS score
FROM memory_edges e
LEFT JOIN chunks c ON c.path = e.src_path
WHERE e.dst_path = ?
GROUP BY e.src_path, e.edge_type
LIMIT ?;`, path, limit)
	if err != nil {
		return nil, err
	}

	merged := make(map[string]RelatedMatch, len(outgoing)+len(incoming))
	merge := func(items []RelatedMatch) {
		for _, item := range items {
			if utils.IsBlank(item.Path) || item.Path == path {
				continue
			}
			key := item.Path + "|" + item.Anchor + "|" + item.EdgeType
			existing, ok := merged[key]
			if !ok || item.Score > existing.Score {
				merged[key] = item
			}
		}
	}
	merge(outgoing)
	merge(incoming)

	if len(merged) == 0 {
		return nil, nil
	}
	results := make([]RelatedMatch, 0, len(merged))
	for _, item := range merged {
		results = append(results, item)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Path < results[j].Path
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (s *IndexStore) fetchRelatedRows(ctx context.Context, query string, args ...any) ([]RelatedMatch, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []RelatedMatch
	for rows.Next() {
		var m RelatedMatch
		if err := rows.Scan(&m.Path, &m.Anchor, &m.EdgeType, &m.StartLine, &m.EndLine, &m.Text, &m.Score); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return matches, nil
}

func deleteByPathTx(ctx context.Context, tx *sql.Tx, path string) error {
	if err := deleteBySourcePathTx(ctx, tx, path); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_edges WHERE dst_path = ?;`, path); err != nil && !isMissingTable(err) {
		return err
	}
	return nil
}

func deleteBySourcePathTx(ctx context.Context, tx *sql.Tx, path string) error {
	stmts := []string{
		`DELETE FROM chunks_vec WHERE rowid IN (SELECT id FROM chunks WHERE path = ?);`,
		`DELETE FROM chunks_fts WHERE rowid IN (SELECT id FROM chunks WHERE path = ?);`,
		`DELETE FROM chunks WHERE path = ?;`,
		`DELETE FROM memory_edges WHERE src_path = ?;`,
	}
	for _, q := range stmts {
		if _, err := tx.ExecContext(ctx, q, path); err != nil && !isMissingTable(err) {
			return err
		}
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
