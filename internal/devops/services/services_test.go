package services

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"alex/internal/devops"
	"alex/internal/devops/health"
	devlog "alex/internal/devops/log"
	"alex/internal/devops/port"
	"alex/internal/devops/process"
)

const (
	devopsHelperEnv     = "ALEX_DEVOPS_TEST_HELPER"
	devopsHelperKindEnv = "ALEX_DEVOPS_TEST_HELPER_KIND"
)

func TestMain(m *testing.M) {
	if envValue(devopsHelperEnv) == "1" {
		runDevopsHelperProcess()
		return
	}
	os.Exit(m.Run())
}

func TestBackendServiceBuildCreatesExecutableStaging(t *testing.T) {
	t.Setenv("CGO_ENABLED", "")

	svc, projectDir, _ := newTestBackendServiceFixture(t)
	writeFakeGoToolchain(t, projectDir)

	staging, err := svc.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if staging != svc.stagingPath() {
		t.Fatalf("Build() staging path = %q, want %q", staging, svc.stagingPath())
	}

	info, err := os.Stat(staging)
	if err != nil {
		t.Fatalf("staging artifact missing: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("staging artifact is not executable: mode=%v", info.Mode())
	}

	if _, err := os.Stat(svc.stampPath()); err != nil {
		t.Fatalf("expected stamp file after build: %v", err)
	}

	if err := svc.Promote(staging); err != nil {
		t.Fatalf("Promote() error: %v", err)
	}

	prodInfo, err := os.Stat(svc.config.OutputBin)
	if err != nil {
		t.Fatalf("promoted binary missing: %v", err)
	}
	if prodInfo.Mode()&0o111 == 0 {
		t.Fatalf("promoted binary is not executable: mode=%v", prodInfo.Mode())
	}
}

func TestBackendServiceStartStopLifecycle(t *testing.T) {
	svc, _, _ := newTestBackendServiceFixture(t)
	svc.config.OutputBin = os.Args[0]
	svc.skipNextBuild = true

	t.Setenv(devopsHelperEnv, "1")
	t.Setenv(devopsHelperKindEnv, "backend")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	if state := svc.State(); state != devops.StateHealthy {
		t.Fatalf("backend state after Start = %s, want %s", state, devops.StateHealthy)
	}

	waitForRunningState(t, func() (bool, int) { return svc.pm.IsRunning("backend") }, true)

	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	if state := svc.State(); state != devops.StateStopped {
		t.Fatalf("backend state after Stop = %s, want %s", state, devops.StateStopped)
	}

	waitForRunningState(t, func() (bool, int) { return svc.pm.IsRunning("backend") }, false)
}

func TestNewWebServiceStartStopLifecycle(t *testing.T) {
	webSvc, webDir, _ := newTestWebServiceFixture(t)
	binDir := filepath.Join(webDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	writeFakeNPM(t, filepath.Join(binDir, "npm"), os.Args[0])

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+envValue("PATH"))
	t.Setenv(devopsHelperEnv, "1")
	t.Setenv(devopsHelperKindEnv, "web")

	if state := webSvc.State(); state != devops.StateStopped {
		t.Fatalf("initial web state = %s, want %s", state, devops.StateStopped)
	}

	lockFile := filepath.Join(webDir, ".next", "dev", "lock")
	if err := os.MkdirAll(filepath.Dir(lockFile), 0o755); err != nil {
		t.Fatalf("mkdir lock dir: %v", err)
	}
	if err := os.WriteFile(lockFile, []byte("stale-lock"), 0o644); err != nil {
		t.Fatalf("write lock file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := webSvc.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	if state := webSvc.State(); state != devops.StateHealthy {
		t.Fatalf("web state after Start = %s, want %s", state, devops.StateHealthy)
	}

	waitForRunningState(t, func() (bool, int) { return webSvc.pm.IsRunning("web") }, true)

	if err := os.WriteFile(lockFile, []byte("active-lock"), 0o644); err != nil {
		t.Fatalf("rewrite lock file: %v", err)
	}

	if err := webSvc.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	if state := webSvc.State(); state != devops.StateStopped {
		t.Fatalf("web state after Stop = %s, want %s", state, devops.StateStopped)
	}
	if _, err := os.Stat(lockFile); !os.IsNotExist(err) {
		t.Fatalf("expected lock file removed on Stop, stat err=%v", err)
	}

	waitForRunningState(t, func() (bool, int) { return webSvc.pm.IsRunning("web") }, false)
}

func TestWebServiceCleanupOrphanProcesses(t *testing.T) {
	webSvc, webDir, _ := newTestWebServiceFixture(t)

	binDir := filepath.Join(webDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	npmPath := filepath.Join(binDir, "npm")
	writeOrphanNPM(t, npmPath)

	cmd := exec.Command(npmPath, "--prefix", webDir, "run", "dev")
	cmd.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+envValue("PATH"))
	if err := cmd.Start(); err != nil {
		t.Fatalf("start orphan npm process: %v", err)
	}
	waitResult := make(chan error, 1)
	go func() {
		waitResult <- cmd.Wait()
	}()
	t.Cleanup(func() {
		_ = syscall.Kill(cmd.Process.Pid, syscall.SIGTERM)
		select {
		case <-waitResult:
		default:
		}
	})

	waitForProcessAlive(t, cmd.Process.Pid, true)

	webSvc.cleanupOrphanWebProcesses()

	select {
	case err := <-waitResult:
		if err != nil && !strings.Contains(err.Error(), "signal: terminated") {
			t.Fatalf("orphan wait error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for orphan process %d to exit", cmd.Process.Pid)
	}
}

func newTestBackendServiceFixture(t *testing.T) (*BackendService, string, *bytesBufferSectionWriter) {
	t.Helper()

	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(projectDir, "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir scripts dir: %v", err)
	}
	logDir := filepath.Join(root, "logs")
	pidDir := filepath.Join(root, "pids")

	writer := newBytesBufferSectionWriter()
	pm := process.NewManager(pidDir, logDir)
	pa := port.NewAllocator()
	hc := health.NewChecker()
	cfg := BackendConfig{
		Port:       0,
		OutputBin:  filepath.Join(root, "alex-server"),
		ProjectDir: projectDir,
		LogDir:     logDir,
		CGOMode:    "off",
	}

	return NewBackendService(pm, pa, hc, writer.SectionWriter, cfg), projectDir, writer
}

func newTestWebServiceFixture(t *testing.T) (*WebService, string, *bytesBufferSectionWriter) {
	t.Helper()

	root := t.TempDir()
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(filepath.Join(webDir, ".next", "dev"), 0o755); err != nil {
		t.Fatalf("mkdir web dev dir: %v", err)
	}
	logDir := filepath.Join(root, "logs")
	pidDir := filepath.Join(root, "pids")

	writer := newBytesBufferSectionWriter()
	pm := process.NewManager(pidDir, logDir)
	pa := port.NewAllocator()
	hc := health.NewChecker()
	cfg := WebConfig{
		Port:       0,
		WebDir:     webDir,
		ServerPort: 18123,
		AutoHeal:   false,
	}

	return NewWebService(pm, pa, hc, writer.SectionWriter, cfg), webDir, writer
}

type bytesBufferSectionWriter struct {
	*devlog.SectionWriter
	mu  sync.Mutex
	buf strings.Builder
}

func newBytesBufferSectionWriter() *bytesBufferSectionWriter {
	w := &bytesBufferSectionWriter{}
	w.SectionWriter = devlog.NewSectionWriter(w, false)
	return w
}

func (w *bytesBufferSectionWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func writeFakeGoToolchain(t *testing.T, projectDir string) {
	t.Helper()

	script := `#!/bin/sh
set -eu
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    out="$2"
    shift 2
    continue
  fi
  shift
done
if [ -z "$out" ]; then
  echo "missing -o" >&2
  exit 1
fi
cat >"$out" <<'EOF'
#!/bin/sh
exit 0
EOF
chmod +x "$out"
`
	path := filepath.Join(projectDir, "scripts", "go-with-toolchain.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake go toolchain: %v", err)
	}
}

func writeFakeNPM(t *testing.T, path, helperBinary string) {
	t.Helper()

	script := fmt.Sprintf(`#!/bin/sh
exec %q
`, helperBinary)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake npm: %v", err)
	}
}

func writeOrphanNPM(t *testing.T, path string) {
	t.Helper()

	script := `#!/bin/sh
trap 'exit 0' TERM INT
while true; do
  sleep 1
done
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write orphan npm: %v", err)
	}
}

func runDevopsHelperProcess() {
	portValue := strings.TrimSpace(envValue("PORT"))
	if portValue == "" {
		fmt.Fprintln(os.Stderr, "PORT not set for devops helper")
		os.Exit(2)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:    "127.0.0.1:" + portValue,
		Handler: mux,
	}

	go func() {
		_, _ = fmt.Fprint(os.Stdout, "")
		if err := srv.ListenAndServe(); err != nil && !errorsIs(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(3)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signalNotify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	os.Exit(0)
}

func waitForRunningState(t *testing.T, check func() (bool, int), want bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		running, _ := check()
		if running == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	running, pid := check()
	t.Fatalf("running state = %v pid=%d, want %v", running, pid, want)
}

func waitForProcessAlive(t *testing.T, pid int, want bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		alive := processAlive(pid)
		if alive == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("process %d alive=%v, want %v", pid, processAlive(pid), want)
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func signalNotify(ch chan<- os.Signal, sig ...os.Signal) {
	signal.Notify(ch, sig...)
}

func errorsIs(err, target error) bool {
	return err == target
}

func envValue(key string) string {
	prefix := key + "="
	for _, entry := range os.Environ() {
		if value, ok := strings.CutPrefix(entry, prefix); ok {
			return value
		}
	}
	return ""
}
