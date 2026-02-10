package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
)

// OrphanedBridge represents a bridge subprocess that was left behind by a
// previous Go process instance.
type OrphanedBridge struct {
	TaskID     string
	PID        int
	OutputFile string
	DoneFile   string
	IsRunning  bool // Process is still alive (signal 0 test).
	HasDone    bool // .done sentinel exists.
}

// DetectOrphanedBridges scans the bridge output directory for orphaned bridge
// subprocesses. Each task gets a subdirectory under {workDir}/.elephant/bridge/.
func DetectOrphanedBridges(workDir string) []OrphanedBridge {
	baseDir := filepath.Join(workDir, ".elephant", "bridge")
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil
	}

	var orphans []OrphanedBridge
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		taskID := entry.Name()
		taskDir := filepath.Join(baseDir, taskID)

		outputFile := filepath.Join(taskDir, "output.jsonl")
		statusFile := filepath.Join(taskDir, "status.json")
		doneFile := filepath.Join(taskDir, ".done")

		// Must have an output file to be a valid bridge dir.
		if _, err := os.Stat(outputFile); err != nil {
			continue
		}

		orphan := OrphanedBridge{
			TaskID:     taskID,
			OutputFile: outputFile,
			DoneFile:   doneFile,
		}

		// Check .done sentinel.
		if _, err := os.Stat(doneFile); err == nil {
			orphan.HasDone = true
		}

		// Read PID from status file.
		orphan.PID = readPIDFromStatus(statusFile)

		// Check if process is still running.
		if orphan.PID > 0 {
			orphan.IsRunning = isProcessAlive(orphan.PID)
		}

		orphans = append(orphans, orphan)
	}

	return orphans
}

// CleanupBridgeDir removes the bridge output directory for a task.
func CleanupBridgeDir(workDir, taskID string) error {
	dir := bridgeOutputDir(workDir, taskID)
	return os.RemoveAll(dir)
}

// readPIDFromStatus reads the PID from a status.json file.
func readPIDFromStatus(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var status struct {
		PID int `json:"pid"`
	}
	if err := json.Unmarshal(data, &status); err != nil {
		return 0
	}
	return status.PID
}

// isProcessAlive checks whether a process is still running by sending signal 0.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 doesn't actually send a signal — it just checks if the
	// process exists and we have permission to signal it.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
