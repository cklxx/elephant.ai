package bridge

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// OutputReader tails a JSONL output file written by a detached bridge subprocess.
// It emits SDKEvents on a channel, supporting both live tailing and offset-based
// resume after process restart.
type OutputReader struct {
	path     string
	donePath string // .done sentinel file path
	offset   int64  // current read position
	mu       sync.Mutex
}

// NewOutputReader creates a reader for the given output file.
// donePath is the path to the .done sentinel file that signals bridge completion.
func NewOutputReader(path, donePath string) *OutputReader {
	return &OutputReader{
		path:     path,
		donePath: donePath,
	}
}

// SetOffset sets the starting read offset for resume.
func (r *OutputReader) SetOffset(offset int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.offset = offset
}

// Offset returns the current read offset.
func (r *OutputReader) Offset() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.offset
}

// Read tails the output file and emits SDKEvents on the returned channel.
// It blocks until the context is cancelled, the .done sentinel appears, or
// the file reaches EOF with no more writes expected.
//
// The channel is closed when reading is complete.
func (r *OutputReader) Read(ctx context.Context) <-chan SDKEvent {
	ch := make(chan SDKEvent, 64)
	go r.tailLoop(ctx, ch)
	return ch
}

// tailLoop is the main tailing goroutine.
func (r *OutputReader) tailLoop(ctx context.Context, ch chan<- SDKEvent) {
	defer close(ch)

	// Wait for the file to exist.
	var f *os.File
	for {
		var err error
		f, err = os.Open(r.path)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	defer f.Close()

	// Seek to saved offset for resume.
	r.mu.Lock()
	offset := r.offset
	r.mu.Unlock()
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			// If seek fails, start from beginning.
			offset = 0
		}
	}

	reader := bufio.NewReader(f)
	pollInterval := 200 * time.Millisecond
	idleCount := 0

	for {
		if ctx.Err() != nil {
			return
		}

		line, err := reader.ReadBytes('\n')
		if err == nil {
			// Got a full line.
			idleCount = 0
			newOffset := offset + int64(len(line))
			ev, parseErr := ParseSDKEvent(line)
			if parseErr == nil {
				select {
				case ch <- ev:
				case <-ctx.Done():
					return
				}
			}
			offset = newOffset
			r.mu.Lock()
			r.offset = offset
			r.mu.Unlock()
			continue
		}

		if err != io.EOF {
			// Hard read error — stop.
			return
		}

		// EOF — check if done.
		if r.isDone() {
			// Read any remaining partial data after sentinel.
			// Re-read from current position to catch final writes.
			if len(line) > 0 {
				ev, parseErr := ParseSDKEvent(line)
				if parseErr == nil {
					select {
					case ch <- ev:
					case <-ctx.Done():
						return
					}
				}
			}
			return
		}

		// Not done yet — poll.
		idleCount++
		wait := pollInterval
		// Back off after many idle polls (up to 2s).
		if idleCount > 20 {
			wait = 2 * time.Second
		} else if idleCount > 5 {
			wait = 500 * time.Millisecond
		}

		select {
		case <-time.After(wait):
			// Re-seek to current offset to pick up new data written by
			// the bridge process (the OS may cache the EOF state).
			_, _ = f.Seek(offset, io.SeekStart)
			reader.Reset(f)
		case <-ctx.Done():
			return
		}
	}
}

// isDone checks whether the .done sentinel file exists.
func (r *OutputReader) isDone() bool {
	if r.donePath == "" {
		return false
	}
	_, err := os.Stat(r.donePath)
	return err == nil
}

// bridgeOutputDir returns the bridge output directory for a task.
func bridgeOutputDir(workDir, taskID string) string {
	return fmt.Sprintf("%s/.elephant/bridge/%s", workDir, taskID)
}

// bridgeOutputFile returns the JSONL output file path for a task.
func bridgeOutputFile(workDir, taskID string) string {
	return fmt.Sprintf("%s/output.jsonl", bridgeOutputDir(workDir, taskID))
}

// bridgeStatusFile returns the status file path for a task.
func bridgeStatusFile(workDir, taskID string) string {
	return fmt.Sprintf("%s/status.json", bridgeOutputDir(workDir, taskID))
}

// bridgeDoneFile returns the .done sentinel file path for a task.
func bridgeDoneFile(workDir, taskID string) string {
	return fmt.Sprintf("%s/.done", bridgeOutputDir(workDir, taskID))
}
