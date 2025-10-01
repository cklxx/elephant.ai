package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Manager handles file backups and rollback operations
type Manager struct {
	backupDir     string
	retentionDays int
	maxSizeMB     int
	sessionID     string
}

// BackupInfo contains metadata about a backup
type BackupInfo struct {
	BackupID     string    `json:"backup_id"`
	OriginalPath string    `json:"original_path"`
	BackupPath   string    `json:"backup_path"`
	SessionID    string    `json:"session_id"`
	Timestamp    time.Time `json:"timestamp"`
	FileSize     int64     `json:"file_size"`
	Operation    string    `json:"operation"` // "edit", "write", "delete"
}

// NewManager creates a new backup manager
func NewManager(backupDir string, sessionID string, retentionDays, maxSizeMB int) (*Manager, error) {
	if backupDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		backupDir = filepath.Join(homeDir, ".alex", "backups")
	}

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	return &Manager{
		backupDir:     backupDir,
		retentionDays: retentionDays,
		maxSizeMB:     maxSizeMB,
		sessionID:     sessionID,
	}, nil
}

// CreateBackup backs up a file before modification
func (m *Manager) CreateBackup(originalPath, operation string) (*BackupInfo, error) {
	// Read original file
	content, err := os.ReadFile(originalPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, create empty backup for new file tracking
			content = []byte{}
		} else {
			return nil, fmt.Errorf("failed to read original file: %w", err)
		}
	}

	// Check size limit
	sizeMB := len(content) / (1024 * 1024)
	if sizeMB > m.maxSizeMB {
		return nil, fmt.Errorf("file too large (%d MB > %d MB limit)", sizeMB, m.maxSizeMB)
	}

	// Generate backup ID and path
	backupID := m.generateBackupID(originalPath)
	backupPath := m.getBackupPath(backupID, originalPath)

	// Create backup directory structure
	backupFileDir := filepath.Dir(backupPath)
	if err := os.MkdirAll(backupFileDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Write backup file
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return nil, fmt.Errorf("failed to write backup: %w", err)
	}

	// Create backup metadata
	info := &BackupInfo{
		BackupID:     backupID,
		OriginalPath: originalPath,
		BackupPath:   backupPath,
		SessionID:    m.sessionID,
		Timestamp:    time.Now(),
		FileSize:     int64(len(content)),
		Operation:    operation,
	}

	// Save metadata
	if err := m.saveMetadata(info); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	return info, nil
}

// RestoreBackup restores a file from backup
func (m *Manager) RestoreBackup(backupID string) error {
	// Load metadata
	info, err := m.loadMetadata(backupID)
	if err != nil {
		return fmt.Errorf("failed to load backup metadata: %w", err)
	}

	// Read backup content
	content, err := os.ReadFile(info.BackupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Create parent directories if needed
	dir := filepath.Dir(info.OriginalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Restore file
	if len(content) == 0 && (info.Operation == "edit" || info.Operation == "create") {
		// This was a new file creation, remove it
		if err := os.Remove(info.OriginalPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove file: %w", err)
		}
	} else {
		// Restore original content
		if err := os.WriteFile(info.OriginalPath, content, 0644); err != nil {
			return fmt.Errorf("failed to restore file: %w", err)
		}
	}

	return nil
}

// ListBackups lists all backups for the current session
func (m *Manager) ListBackups() ([]*BackupInfo, error) {
	sessionDir := filepath.Join(m.backupDir, m.sessionID)
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		return []*BackupInfo{}, nil
	}

	var backups []*BackupInfo

	err := filepath.Walk(sessionDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".meta.json") {
			backupID := strings.TrimSuffix(filepath.Base(path), ".meta.json")
			backupInfo, err := m.loadMetadata(backupID)
			if err != nil {
				// Skip corrupted metadata
				return nil
			}
			backups = append(backups, backupInfo)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// GetLastBackup returns the most recent backup for the current session
func (m *Manager) GetLastBackup() (*BackupInfo, error) {
	backups, err := m.ListBackups()
	if err != nil {
		return nil, err
	}

	if len(backups) == 0 {
		return nil, fmt.Errorf("no backups found")
	}

	return backups[0], nil
}

// CleanupOldBackups removes backups older than retention period
func (m *Manager) CleanupOldBackups() error {
	cutoffTime := time.Now().AddDate(0, 0, -m.retentionDays)

	err := filepath.Walk(m.backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.ModTime().Before(cutoffTime) {
			// Remove old backup files
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove old backup: %w", err)
			}
		}

		return nil
	})

	return err
}

// GetBackupSize returns the total size of all backups in MB
func (m *Manager) GetBackupSize() (int64, error) {
	var totalSize int64

	err := filepath.Walk(m.backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			totalSize += info.Size()
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return totalSize / (1024 * 1024), nil
}

// generateBackupID creates a unique backup ID
func (m *Manager) generateBackupID(originalPath string) string {
	// Use hash of path + timestamp for uniqueness
	timestamp := time.Now().Format("20060102-150405")
	hash := sha256.Sum256([]byte(originalPath + timestamp))
	hashStr := hex.EncodeToString(hash[:])[:16]
	return fmt.Sprintf("%s-%s", timestamp, hashStr)
}

// getBackupPath returns the full path for a backup file
func (m *Manager) getBackupPath(backupID, originalPath string) string {
	// Create a safe filename from the original path
	safePath := strings.ReplaceAll(originalPath, string(filepath.Separator), "_")
	safePath = strings.ReplaceAll(safePath, ":", "_")

	return filepath.Join(m.backupDir, m.sessionID, backupID, safePath)
}

// getMetadataPath returns the path for backup metadata
func (m *Manager) getMetadataPath(backupID string) string {
	return filepath.Join(m.backupDir, m.sessionID, backupID, backupID+".meta.json")
}

// saveMetadata saves backup metadata to disk
func (m *Manager) saveMetadata(info *BackupInfo) error {
	metadataPath := m.getMetadataPath(info.BackupID)

	// Create directory if needed
	dir := filepath.Dir(metadataPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0644)
}

// loadMetadata loads backup metadata from disk
func (m *Manager) loadMetadata(backupID string) (*BackupInfo, error) {
	metadataPath := m.getMetadataPath(backupID)

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}

	var info BackupInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// DeleteBackup removes a specific backup
func (m *Manager) DeleteBackup(backupID string) error {
	backupDir := filepath.Join(m.backupDir, m.sessionID, backupID)
	return os.RemoveAll(backupDir)
}
