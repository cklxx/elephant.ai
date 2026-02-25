package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterh/liner"
)

var errPromptAborted = errors.New("prompt aborted")

type linePrompter interface {
	Prompt() (string, bool, error)
	AppendHistory(entry string)
	Close() error
}

type bufferedPrompter struct {
	reader       *bufio.Reader
	prompt       string
	out          io.Writer
	promptActive bool
}

func newBufferedPrompter(reader *bufio.Reader, prompt string, out io.Writer, promptActive bool) *bufferedPrompter {
	return &bufferedPrompter{
		reader:       reader,
		prompt:       prompt,
		out:          out,
		promptActive: promptActive,
	}
}

func (p *bufferedPrompter) Prompt() (string, bool, error) {
	if p == nil || p.reader == nil {
		return "", false, nil
	}
	if p.promptActive && p.out != nil && p.prompt != "" {
		fmt.Fprint(p.out, p.prompt)
	}
	return readLine(p.reader)
}

func (p *bufferedPrompter) AppendHistory(string) {}

func (p *bufferedPrompter) Close() error { return nil }

type linerPrompter struct {
	state       *liner.State
	prompt      string
	historyPath string
}

func newLinerPrompter(prompt string, historyPath string, errOut io.Writer) (*linerPrompter, error) {
	state := liner.NewLiner()
	state.SetCtrlCAborts(true)
	state.SetMultiLineMode(false)
	state.SetCompleter(lineCommandCompleter)

	if historyPath != "" {
		if err := ensureHistoryDir(historyPath); err != nil {
			printHistoryWarning(errOut, "history directory", err)
		} else if file, err := os.Open(historyPath); err == nil {
			if _, err := state.ReadHistory(file); err != nil {
				printHistoryWarning(errOut, "read history", err)
			}
			_ = file.Close()
		} else if !os.IsNotExist(err) {
			printHistoryWarning(errOut, "open history", err)
		}
	}

	return &linerPrompter{
		state:       state,
		prompt:      prompt,
		historyPath: historyPath,
	}, nil
}

func (p *linerPrompter) Prompt() (string, bool, error) {
	if p == nil || p.state == nil {
		return "", false, nil
	}
	line, err := p.state.Prompt(p.prompt)
	if err == nil {
		return line, true, nil
	}
	if errors.Is(err, liner.ErrPromptAborted) {
		return "", false, errPromptAborted
	}
	if errors.Is(err, io.EOF) {
		return "", false, nil
	}
	return "", false, err
}

func (p *linerPrompter) AppendHistory(entry string) {
	if p == nil || p.state == nil {
		return
	}
	if strings.TrimSpace(entry) == "" {
		return
	}
	p.state.AppendHistory(entry)
}

func (p *linerPrompter) Close() error {
	if p == nil || p.state == nil {
		return nil
	}
	if p.historyPath != "" {
		if err := ensureHistoryDir(p.historyPath); err != nil {
			return p.state.Close()
		}
		file, err := os.OpenFile(p.historyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err == nil {
			if _, err := p.state.WriteHistory(file); err != nil {
				printHistoryWarning(os.Stderr, "write history", err)
			}
			_ = file.Close()
		} else {
			printHistoryWarning(os.Stderr, "open history for write", err)
		}
	}
	return p.state.Close()
}

func historyFilePath(container *Container) string {
	if container == nil {
		return ""
	}
	baseDir := ""
	if container.Container != nil {
		if sessionDir := container.SessionDir(); sessionDir != "" {
			baseDir = filepath.Dir(sessionDir)
		}
	}
	if baseDir == "" || baseDir == "." {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			baseDir = filepath.Join(home, ".alex")
		}
	}
	if baseDir == "" {
		return ""
	}
	return filepath.Join(baseDir, "history")
}

func ensureHistoryDir(historyPath string) error {
	if historyPath == "" {
		return nil
	}
	return os.MkdirAll(filepath.Dir(historyPath), 0o700)
}

func printHistoryWarning(out io.Writer, action string, err error) {
	if out == nil || err == nil {
		return
	}
	fmt.Fprintf(out, "Warning: %s failed: %v\n", action, err)
}

func lineCommandCompleter(line string) []string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "/") {
		var matches []string
		for _, cmd := range lineModeCommands() {
			if strings.HasPrefix(cmd, trimmed) {
				matches = append(matches, cmd)
			}
		}
		return matches
	}
	return nil
}
