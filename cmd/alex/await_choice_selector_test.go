package main

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestNormalizeChoiceOptions(t *testing.T) {
	t.Parallel()

	options := normalizeChoiceOptions([]string{"  dev ", "staging", "", "dev", "prod"})
	if len(options) != 3 {
		t.Fatalf("expected 3 options, got %#v", options)
	}
	if options[0] != "dev" || options[1] != "staging" || options[2] != "prod" {
		t.Fatalf("unexpected normalized options: %#v", options)
	}
}

func TestReadSelectorKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want selectorKey
	}{
		{name: "arrow up", in: []byte{27, '[', 'A'}, want: selectorKeyUp},
		{name: "arrow down", in: []byte{27, '[', 'B'}, want: selectorKeyDown},
		{name: "enter", in: []byte{'\n'}, want: selectorKeyEnter},
		{name: "ctrl c", in: []byte{3}, want: selectorKeyAbort},
		{name: "j", in: []byte{'j'}, want: selectorKeyDown},
		{name: "k", in: []byte{'k'}, want: selectorKeyUp},
		{name: "unknown", in: []byte{'x'}, want: selectorKeyUnknown},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.in))
			got, err := readSelectorKey(reader)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestAwaitChoiceSelectorNonInteractiveSkipsSelection(t *testing.T) {
	t.Parallel()

	sel := newAwaitChoiceSelector(strings.NewReader(""), io.Discard, false)
	choice, ok, err := sel.Select("Pick one", []string{"A", "B"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok || choice != "" {
		t.Fatalf("expected no selection in non-interactive mode, got ok=%v choice=%q", ok, choice)
	}
}
