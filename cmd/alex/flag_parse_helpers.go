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

func formatBufferedFlagParseError(err error, flagBuf *bytes.Buffer) error {
	return fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))
}
