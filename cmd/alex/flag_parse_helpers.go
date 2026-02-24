package main

import (
	"bytes"
	"flag"
	"fmt"
	"strings"
)

func newBufferedFlagSet(name string) (*flag.FlagSet, *bytes.Buffer) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)
	return fs, &flagBuf
}

func parseBufferedFlagSet(fs *flag.FlagSet, flagBuf *bytes.Buffer, args []string) error {
	if err := fs.Parse(args); err != nil {
		return formatBufferedFlagParseError(err, flagBuf)
	}
	return nil
}

func formatBufferedFlagParseError(err error, flagBuf *bytes.Buffer) error {
	if err == nil {
		return nil
	}
	if flagBuf == nil {
		return err
	}
	return fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))
}
