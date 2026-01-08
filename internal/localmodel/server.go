package localmodel

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"alex/internal/logging"
)

type ServerManager struct {
	mu       sync.Mutex
	starting bool
	started  bool
	startErr error
	startCh  chan struct{}
	cmd      *exec.Cmd
}

var (
	defaultManager *ServerManager
	managerOnce    sync.Once
)

func DefaultManager() *ServerManager {
	managerOnce.Do(func() {
		defaultManager = &ServerManager{}
	})
	return defaultManager
}

func (m *ServerManager) Ensure(ctx context.Context, logger logging.Logger, baseURL string) error {
	if baseURL == "" {
		baseURL = BaseURL
	}
	log := logging.OrNop(logger)

	healthCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if checkHealth(healthCtx, baseURL) {
		return nil
	}

	m.mu.Lock()
	if m.starting {
		ch := m.startCh
		m.mu.Unlock()
		select {
		case <-ch:
			return m.startErr
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if m.started {
		m.mu.Unlock()
		return nil
	}
	m.starting = true
	m.startCh = make(chan struct{})
	m.mu.Unlock()

	err := m.start(ctx, log, baseURL)

	m.mu.Lock()
	m.starting = false
	m.startErr = err
	if err == nil {
		m.started = true
	}
	close(m.startCh)
	m.mu.Unlock()

	return err
}

func (m *ServerManager) start(ctx context.Context, logger logging.Logger, baseURL string) error {
	root, err := findRepoRoot()
	if err != nil {
		logger.Warn("Failed to locate repo root, using current directory: %v", err)
		root, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve working dir: %w", err)
		}
	}

	modelPath := filepath.Join(root, RelativeModelPath)
	if err := ensureModel(ctx, logger, modelPath); err != nil {
		return err
	}

	templatePath := filepath.Join(root, RelativeTemplatePath)
	if _, err := os.Stat(templatePath); err != nil {
		return fmt.Errorf("chat template missing: %s", templatePath)
	}

	binPath, err := ensureLlamaServer(ctx, logger, root)
	if err != nil {
		return err
	}

	host, port := splitHostPort(baseURL)

	args := []string{
		"--host", host,
		"--port", port,
		"--alias", ModelID,
		"-m", modelPath,
		"-c", fmt.Sprintf("%d", DefaultContextSize),
		"--jinja",
		"--chat-template-file", templatePath,
		"--log-disable",
	}

	logFile, err := openLogFile(root)
	if err != nil {
		return fmt.Errorf("open llama-server log: %w", err)
	}

	cmd := exec.Command(binPath, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start llama-server: %w", err)
	}

	m.mu.Lock()
	m.cmd = cmd
	m.mu.Unlock()

	if err := waitForHealth(ctx, baseURL, 20*time.Second); err != nil {
		return err
	}

	logger.Info("Local inference server ready: %s", baseURL)
	return nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found")
		}
		dir = parent
	}
}

func ensureModel(ctx context.Context, logger logging.Logger, modelPath string) error {
	if info, err := os.Stat(modelPath); err == nil && info.Size() >= MinModelSizeBytes {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(modelPath), 0o755); err != nil {
		return fmt.Errorf("create model directory: %w", err)
	}
	logger.Info("Downloading FunctionGemma weights to %s", modelPath)
	return downloadFile(ctx, DownloadURL, modelPath)
}

func ensureLlamaServer(ctx context.Context, logger logging.Logger, root string) (string, error) {
	if path, err := exec.LookPath(LlamaServerBinaryName); err == nil {
		return path, nil
	}
	asset, err := llamaAssetName()
	if err != nil {
		return "", err
	}
	baseDir := filepath.Join(root, ".toolchains", "llama.cpp", DefaultLlamaRelease, runtime.GOOS+"-"+runtime.GOARCH)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", fmt.Errorf("create llama.cpp toolchain dir: %w", err)
	}
	targetPath := filepath.Join(baseDir, LlamaServerBinaryName)
	if info, err := os.Stat(targetPath); err == nil && info.Mode().Perm()&0o111 != 0 && hasLlamaServerDeps(baseDir) {
		return targetPath, nil
	}

	archivePath := filepath.Join(baseDir, asset)
	urls := []string{
		fmt.Sprintf("https://github.com/ggml-org/llama.cpp/releases/download/%s/%s", DefaultLlamaRelease, asset),
		fmt.Sprintf("https://github.com/ggerganov/llama.cpp/releases/download/%s/%s", DefaultLlamaRelease, asset),
	}
	logger.Info("Downloading llama.cpp server (%s)", asset)
	if err := downloadFileWithFallback(ctx, urls, archivePath); err != nil {
		return "", err
	}
	if err := extractLlamaServer(archivePath, targetPath); err != nil {
		return "", err
	}
	if err := os.Chmod(targetPath, 0o755); err != nil {
		return "", fmt.Errorf("chmod llama-server: %w", err)
	}
	return targetPath, nil
}

func llamaAssetName() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "llama-" + DefaultLlamaRelease + "-bin-macos-arm64.tar.gz", nil
		}
		if runtime.GOARCH == "amd64" {
			return "llama-" + DefaultLlamaRelease + "-bin-macos-x64.tar.gz", nil
		}
	case "linux":
		if runtime.GOARCH == "amd64" {
			return "llama-" + DefaultLlamaRelease + "-bin-ubuntu-x64.tar.gz", nil
		}
	}
	return "", fmt.Errorf("no prebuilt llama-server for %s/%s; install llama.cpp manually", runtime.GOOS, runtime.GOARCH)
}

func hasLlamaServerDeps(baseDir string) bool {
	switch runtime.GOOS {
	case "darwin":
		return fileExists(filepath.Join(baseDir, "libmtmd.0.dylib"))
	case "linux":
		return fileExists(filepath.Join(baseDir, "libmtmd.so.0")) || fileExists(filepath.Join(baseDir, "libmtmd.so"))
	default:
		return true
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func extractLlamaServer(archivePath, targetPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	baseDir := filepath.Dir(targetPath)
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		parts := strings.SplitN(name, "/", 2)
		if len(parts) == 2 {
			name = parts[1]
		}
		if name == "" || strings.HasPrefix(name, "..") {
			continue
		}
		outPath := filepath.Join(baseDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("create llama.cpp dir: %w", err)
		}
		switch hdr.Typeflag {
		case tar.TypeReg:
			out, err := os.Create(outPath)
			if err != nil {
				return fmt.Errorf("create llama.cpp file: %w", err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return fmt.Errorf("write llama.cpp file: %w", err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("close llama.cpp file: %w", err)
			}
			if filepath.Base(outPath) == LlamaServerBinaryName {
				found = true
			}
		case tar.TypeSymlink:
			_ = os.Remove(outPath)
			if err := os.Symlink(hdr.Linkname, outPath); err != nil {
				return fmt.Errorf("create llama.cpp symlink: %w", err)
			}
		default:
			continue
		}
	}
	if !found {
		return errors.New("llama-server not found in archive")
	}
	return nil
}

func downloadFile(ctx context.Context, url, dest string) error {
	return downloadFileWithFallback(ctx, []string{url}, dest)
}

func downloadFileWithFallback(ctx context.Context, urls []string, dest string) error {
	var lastErr error
	for _, url := range urls {
		if err := downloadFileOnce(ctx, url, dest); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("download failed")
	}
	return lastErr
}

func downloadFileOnce(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "alex-localmodel")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".tmp-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

func splitHostPort(baseURL string) (string, string) {
	base := strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	base = strings.TrimSuffix(base, "/v1")
	base = strings.TrimPrefix(base, "http://")
	base = strings.TrimPrefix(base, "https://")
	hostPort := strings.SplitN(base, "/", 2)[0]
	if hostPort == "" {
		return DefaultServerHost, DefaultServerPort
	}
	if strings.Contains(hostPort, ":") {
		parts := strings.SplitN(hostPort, ":", 2)
		return parts[0], parts[1]
	}
	return hostPort, DefaultServerPort
}

func openLogFile(root string) (*os.File, error) {
	logDir := filepath.Join(root, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(logDir, "local-llama-server.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

func waitForHealth(ctx context.Context, baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		healthCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
		ok := checkHealth(healthCtx, baseURL)
		cancel()
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("local inference server did not become ready at %s", baseURL)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func checkHealth(ctx context.Context, baseURL string) bool {
	url := strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	url = strings.TrimSuffix(url, "/v1")
	if url == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
