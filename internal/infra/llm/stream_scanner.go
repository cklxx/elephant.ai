package llm

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

const (
	streamScannerInitialBuffer = 64 * 1024
)

type streamScanner struct {
	reader *bufio.Reader
	text   string
	err    error
	done   bool
}

func newStreamScanner(reader io.Reader) *streamScanner {
	return &streamScanner{
		reader: bufio.NewReaderSize(reader, streamScannerInitialBuffer),
	}
}

func (s *streamScanner) Scan() bool {
	if s.done {
		return false
	}

	line, err := s.reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			if line == "" {
				s.done = true
				return false
			}
			s.done = true
			s.text = strings.TrimRight(line, "\r\n")
			return true
		}
		s.done = true
		s.err = err
		return false
	}

	s.text = strings.TrimRight(line, "\r\n")
	return true
}

func (s *streamScanner) Text() string {
	return s.text
}

func (s *streamScanner) Err() error {
	return s.err
}
