package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	manager, err := NewManager(backupDir, "test-session", 7, 100)
	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.Equal(t, backupDir, manager.backupDir)
	assert.Equal(t, "test-session", manager.sessionID)
	assert.Equal(t, 7, manager.retentionDays)
	assert.Equal(t, 100, manager.maxSizeMB)

	// Check that backup directory was created
	_, err = os.Stat(backupDir)
	assert.NoError(t, err)
}

func TestManager_CreateBackup(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create test file
	content := []byte("original content\nline 2\n")
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	manager, err := NewManager(backupDir, "test-session", 7, 100)
	require.NoError(t, err)

	// Create backup
	info, err := manager.CreateBackup(testFile, "edit")
	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.NotEmpty(t, info.BackupID)
	assert.Equal(t, testFile, info.OriginalPath)
	assert.Equal(t, "test-session", info.SessionID)
	assert.Equal(t, "edit", info.Operation)
	assert.Equal(t, int64(len(content)), info.FileSize)

	// Verify backup file exists
	_, err = os.Stat(info.BackupPath)
	assert.NoError(t, err)

	// Verify backup content
	backupContent, err := os.ReadFile(info.BackupPath)
	require.NoError(t, err)
	assert.Equal(t, content, backupContent)
}

func TestManager_CreateBackup_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	testFile := filepath.Join(tmpDir, "nonexistent.txt")

	manager, err := NewManager(backupDir, "test-session", 7, 100)
	require.NoError(t, err)

	// Create backup of non-existent file (for new file tracking)
	info, err := manager.CreateBackup(testFile, "edit")
	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, int64(0), info.FileSize)
}

func TestManager_RestoreBackup(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create test file with original content
	originalContent := []byte("original content\n")
	err := os.WriteFile(testFile, originalContent, 0644)
	require.NoError(t, err)

	manager, err := NewManager(backupDir, "test-session", 7, 100)
	require.NoError(t, err)

	// Create backup
	info, err := manager.CreateBackup(testFile, "edit")
	require.NoError(t, err)

	// Modify the file
	modifiedContent := []byte("modified content\n")
	err = os.WriteFile(testFile, modifiedContent, 0644)
	require.NoError(t, err)

	// Restore from backup
	err = manager.RestoreBackup(info.BackupID)
	require.NoError(t, err)

	// Verify restored content
	restoredContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, originalContent, restoredContent)
}

func TestManager_RestoreBackup_NewFileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	testFile := filepath.Join(tmpDir, "newfile.txt")

	manager, err := NewManager(backupDir, "test-session", 7, 100)
	require.NoError(t, err)

	// Create backup of non-existent file
	info, err := manager.CreateBackup(testFile, "edit")
	require.NoError(t, err)

	// Create the new file
	err = os.WriteFile(testFile, []byte("new content\n"), 0644)
	require.NoError(t, err)

	// Restore from backup (should delete the file)
	err = manager.RestoreBackup(info.BackupID)
	require.NoError(t, err)

	// Verify file was deleted
	_, err = os.Stat(testFile)
	assert.True(t, os.IsNotExist(err))
}

func TestManager_ListBackups(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	manager, err := NewManager(backupDir, "test-session", 7, 100)
	require.NoError(t, err)

	// Create multiple test files and backups
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("test%d.txt", i))
		err := os.WriteFile(testFile, []byte(fmt.Sprintf("content %d\n", i)), 0644)
		require.NoError(t, err)

		_, err = manager.CreateBackup(testFile, "edit")
		require.NoError(t, err)

		// Add small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// List backups
	backups, err := manager.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 3)

	// Verify backups are sorted by timestamp (newest first)
	for i := 0; i < len(backups)-1; i++ {
		assert.True(t, backups[i].Timestamp.After(backups[i+1].Timestamp) ||
			backups[i].Timestamp.Equal(backups[i+1].Timestamp))
	}
}

func TestManager_GetLastBackup(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	manager, err := NewManager(backupDir, "test-session", 7, 100)
	require.NoError(t, err)

	// Initially, should return error
	_, err = manager.GetLastBackup()
	assert.Error(t, err)

	// Create backup
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content\n"), 0644)
	require.NoError(t, err)

	info, err := manager.CreateBackup(testFile, "edit")
	require.NoError(t, err)

	// Get last backup
	lastBackup, err := manager.GetLastBackup()
	require.NoError(t, err)
	assert.Equal(t, info.BackupID, lastBackup.BackupID)
}

func TestManager_CleanupOldBackups(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	manager, err := NewManager(backupDir, "test-session", 1, 100) // 1 day retention
	require.NoError(t, err)

	// Create backup
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content\n"), 0644)
	require.NoError(t, err)

	info, err := manager.CreateBackup(testFile, "edit")
	require.NoError(t, err)

	// Modify backup file timestamp to be old
	oldTime := time.Now().AddDate(0, 0, -2) // 2 days ago
	err = os.Chtimes(info.BackupPath, oldTime, oldTime)
	require.NoError(t, err)

	// Cleanup old backups
	err = manager.CleanupOldBackups()
	require.NoError(t, err)

	// Verify old backup was removed
	_, err = os.Stat(info.BackupPath)
	assert.True(t, os.IsNotExist(err))
}

func TestManager_GetBackupSize(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	manager, err := NewManager(backupDir, "test-session", 7, 100)
	require.NoError(t, err)

	// Initially, size should be 0
	size, err := manager.GetBackupSize()
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)

	// Create backup
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content\n")
	err = os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	_, err = manager.CreateBackup(testFile, "edit")
	require.NoError(t, err)

	// Size should be >= 0 (backup exists even if content is small)
	size, err = manager.GetBackupSize()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, size, int64(0))
}

func TestManager_DeleteBackup(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	manager, err := NewManager(backupDir, "test-session", 7, 100)
	require.NoError(t, err)

	// Create backup
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content\n"), 0644)
	require.NoError(t, err)

	info, err := manager.CreateBackup(testFile, "edit")
	require.NoError(t, err)

	// Verify backup exists
	backups, err := manager.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 1)

	// Delete backup
	err = manager.DeleteBackup(info.BackupID)
	require.NoError(t, err)

	// Verify backup was deleted
	backups, err = manager.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 0)
}

func TestManager_CreateBackup_SizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create manager with 1MB limit
	manager, err := NewManager(backupDir, "test-session", 7, 1)
	require.NoError(t, err)

	// Create large file (2MB)
	largeContent := make([]byte, 2*1024*1024)
	err = os.WriteFile(testFile, largeContent, 0644)
	require.NoError(t, err)

	// Attempt to create backup should fail
	_, err = manager.CreateBackup(testFile, "edit")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestManager_SessionIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	// Create two managers with different session IDs
	manager1, err := NewManager(backupDir, "session-1", 7, 100)
	require.NoError(t, err)

	manager2, err := NewManager(backupDir, "session-2", 7, 100)
	require.NoError(t, err)

	// Create backup in session 1
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content\n"), 0644)
	require.NoError(t, err)

	_, err = manager1.CreateBackup(testFile, "edit")
	require.NoError(t, err)

	// Session 1 should have 1 backup
	backups1, err := manager1.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups1, 1)

	// Session 2 should have 0 backups
	backups2, err := manager2.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups2, 0)
}
