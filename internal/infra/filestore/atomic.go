package filestore

import (
	"os"
	"path/filepath"

	jsonx "alex/internal/shared/json"
)

// EnsureDir creates the directory and all parents if they don't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// EnsureParentDir creates the parent directory of filePath.
func EnsureParentDir(filePath string) error {
	return EnsureDir(filepath.Dir(filePath))
}

// AtomicWrite writes data to filePath via a temporary file + rename.
// This prevents partial writes from corrupting the file.
func AtomicWrite(filePath string, data []byte, perm os.FileMode) error {
	if err := EnsureParentDir(filePath); err != nil {
		return err
	}
	tmp := filePath + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, filePath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ReadFileOrEmpty reads a file, returning (nil, nil) if the file doesn't exist.
func ReadFileOrEmpty(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

// ResolvePath resolves a storage path, handling ~ expansion and env variables.
// If configured is empty, defaultPath is used.
func ResolvePath(configured, defaultPath string) string {
	path := configured
	if path == "" {
		path = defaultPath
	}
	if path == "" {
		return path
	}

	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			if len(path) > 1 && path[1] == '/' {
				path = filepath.Join(home, path[2:])
			} else if len(path) == 1 {
				path = home
			} else {
				path = filepath.Join(home, path[1:])
			}
		}
	}

	path = os.ExpandEnv(path)
	return path
}

// MarshalJSONIndent marshals v as indented JSON with a trailing newline.
func MarshalJSONIndent(v any) ([]byte, error) {
	data, err := jsonx.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
