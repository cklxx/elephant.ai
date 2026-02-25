package log

import (
	"fmt"
	"io"
	"os"
)

const (
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorBlue   = "\033[0;34m"
	colorCyan   = "\033[0;36m"
	colorReset  = "\033[0m"
)

// SectionWriter provides structured terminal output with color-coded sections.
type SectionWriter struct {
	w      io.Writer
	colors bool
}

// NewSectionWriter creates a new SectionWriter.
func NewSectionWriter(w io.Writer, colors bool) *SectionWriter {
	if w == nil {
		w = os.Stdout
	}
	return &SectionWriter{w: w, colors: colors}
}

// Section prints a section header.
func (s *SectionWriter) Section(name string) {
	if s.colors {
		fmt.Fprintf(s.w, "\n%s── %s ──%s\n", colorCyan, name, colorReset)
	} else {
		fmt.Fprintf(s.w, "\n── %s ──\n", name)
	}
}

// Info prints an info message.
func (s *SectionWriter) Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if s.colors {
		fmt.Fprintf(s.w, "%s▸%s %s\n", colorBlue, colorReset, msg)
	} else {
		fmt.Fprintf(s.w, "▸ %s\n", msg)
	}
}

// Success prints a success message.
func (s *SectionWriter) Success(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if s.colors {
		fmt.Fprintf(s.w, "%s✓%s %s\n", colorGreen, colorReset, msg)
	} else {
		fmt.Fprintf(s.w, "✓ %s\n", msg)
	}
}

// Warn prints a warning message.
func (s *SectionWriter) Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if s.colors {
		fmt.Fprintf(s.w, "%s⚠%s %s\n", colorYellow, colorReset, msg)
	} else {
		fmt.Fprintf(s.w, "⚠ %s\n", msg)
	}
}

// Error prints an error message to stderr.
func (s *SectionWriter) Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if s.colors {
		fmt.Fprintf(os.Stderr, "%s✗%s %s\n", colorRed, colorReset, msg)
	} else {
		fmt.Fprintf(os.Stderr, "✗ %s\n", msg)
	}
}
