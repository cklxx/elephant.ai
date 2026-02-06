package llm

import (
	"bufio"
	"io"
)

const (
	streamScannerInitialBuffer = 64 * 1024
	streamScannerMaxBuffer     = 512 * 1024
)

func newStreamScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, streamScannerInitialBuffer), streamScannerMaxBuffer)
	return scanner
}
