package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

type awaitChoiceSelector struct {
	in          io.Reader
	out         io.Writer
	interactive bool
}

func newAwaitChoiceSelector(in io.Reader, out io.Writer, interactive bool) *awaitChoiceSelector {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	return &awaitChoiceSelector{in: in, out: out, interactive: interactive}
}

func (s *awaitChoiceSelector) Select(question string, options []string) (string, bool, error) {
	if s == nil {
		return "", false, nil
	}
	trimmedQuestion := strings.TrimSpace(question)
	normalized := normalizeChoiceOptions(options)
	if trimmedQuestion == "" || len(normalized) == 0 {
		return "", false, nil
	}
	if !s.interactive {
		return "", false, nil
	}

	inFile, inOK := s.in.(*os.File)
	outFile, outOK := s.out.(*os.File)
	if !inOK || !outOK {
		return "", false, nil
	}
	if !term.IsTerminal(int(inFile.Fd())) || !term.IsTerminal(int(outFile.Fd())) {
		return "", false, nil
	}
	return s.selectWithArrowKeys(inFile, normalized, trimmedQuestion)
}

func (s *awaitChoiceSelector) selectWithArrowKeys(inFile *os.File, options []string, question string) (string, bool, error) {
	state, err := term.MakeRaw(int(inFile.Fd()))
	if err != nil {
		return "", false, err
	}
	defer func() {
		_ = term.Restore(int(inFile.Fd()), state)
	}()

	if _, err := fmt.Fprintf(s.out, "\n%s\n%s\n", styleGray.Render("Use ↑/↓ and Enter to choose."), question); err != nil {
		return "", false, err
	}
	selected := 0
	if err := renderChoiceRows(s.out, options, selected); err != nil {
		return "", false, err
	}

	reader := bufio.NewReader(inFile)
	for {
		key, err := readSelectorKey(reader)
		if err != nil {
			return "", false, err
		}
		switch key {
		case selectorKeyUp:
			if selected == 0 {
				selected = len(options) - 1
			} else {
				selected--
			}
			if _, err := fmt.Fprintf(s.out, "\033[%dA", len(options)); err != nil {
				return "", false, err
			}
			if err := renderChoiceRows(s.out, options, selected); err != nil {
				return "", false, err
			}
		case selectorKeyDown:
			selected = (selected + 1) % len(options)
			if _, err := fmt.Fprintf(s.out, "\033[%dA", len(options)); err != nil {
				return "", false, err
			}
			if err := renderChoiceRows(s.out, options, selected); err != nil {
				return "", false, err
			}
		case selectorKeyEnter:
			if _, err := fmt.Fprint(s.out, "\n"); err != nil {
				return "", false, err
			}
			return options[selected], true, nil
		case selectorKeyAbort:
			if _, err := fmt.Fprint(s.out, "\n"); err != nil {
				return "", false, err
			}
			return "", false, errPromptAborted
		default:
			continue
		}
	}
}

func normalizeChoiceOptions(options []string) []string {
	if len(options) == 0 {
		return nil
	}
	out := make([]string, 0, len(options))
	seen := make(map[string]struct{})
	for _, raw := range options {
		option := strings.TrimSpace(raw)
		if option == "" {
			continue
		}
		if _, exists := seen[option]; exists {
			continue
		}
		seen[option] = struct{}{}
		out = append(out, option)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func renderChoiceRows(out io.Writer, options []string, selected int) error {
	for i, option := range options {
		if i == selected {
			if _, err := fmt.Fprintf(out, "\033[2K%s %s\n", styleGreen.Render(">"), styleGreen.Render(option)); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(out, "\033[2K  %s\n", option); err != nil {
			return err
		}
	}
	return nil
}

type selectorKey uint8

const (
	selectorKeyUnknown selectorKey = iota
	selectorKeyUp
	selectorKeyDown
	selectorKeyEnter
	selectorKeyAbort
)

func readSelectorKey(reader *bufio.Reader) (selectorKey, error) {
	if reader == nil {
		return selectorKeyUnknown, nil
	}
	b, err := reader.ReadByte()
	if err != nil {
		return selectorKeyUnknown, err
	}
	switch b {
	case 3:
		return selectorKeyAbort, nil
	case '\r', '\n':
		return selectorKeyEnter, nil
	case 'k':
		return selectorKeyUp, nil
	case 'j':
		return selectorKeyDown, nil
	case 27:
		next, err := reader.ReadByte()
		if err != nil || next != '[' {
			return selectorKeyUnknown, nil
		}
		direction, err := reader.ReadByte()
		if err != nil {
			return selectorKeyUnknown, nil
		}
		switch direction {
		case 'A':
			return selectorKeyUp, nil
		case 'B':
			return selectorKeyDown, nil
		default:
			return selectorKeyUnknown, nil
		}
	default:
		return selectorKeyUnknown, nil
	}
}
