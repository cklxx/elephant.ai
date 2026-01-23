package main

import (
	"bufio"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseACPPrompt(t *testing.T) {
	prompt := []any{
		map[string]any{"type": "text", "text": "hello"},
		map[string]any{"type": "resource_link", "uri": "file:///tmp/readme.md", "name": "readme.md"},
		map[string]any{"type": "text", "text": "next"},
	}
	parsed, err := parseACPPrompt(prompt)
	require.NoError(t, err)
	require.Equal(t, "hello\n[readme.md]\nnext", parsed.Text)
	require.Len(t, parsed.Attachments, 1)
	require.Equal(t, "readme.md", parsed.Attachments[0].Name)
	require.Equal(t, "file:///tmp/readme.md", parsed.Attachments[0].URI)
}

func TestReadRPCMessageContentLength(t *testing.T) {
	payload := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload)
	reader := bufio.NewReader(strings.NewReader(input))

	got, usedHeaders, err := readRPCMessage(reader)
	require.NoError(t, err)
	require.True(t, usedHeaders)
	require.Equal(t, payload, string(got))
}

func TestReadRPCMessageLineDelimited(t *testing.T) {
	payload := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	reader := bufio.NewReader(strings.NewReader(payload + "\n"))

	got, usedHeaders, err := readRPCMessage(reader)
	require.NoError(t, err)
	require.False(t, usedHeaders)
	require.Equal(t, payload, string(got))
}
