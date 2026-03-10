package jsonx

import (
	stdjson "encoding/json"
	"fmt"
	"io"

	goccyjson "github.com/goccy/go-json"
)

// Thin wrapper so hot paths can swap JSON implementations in one place.
var (
	marshalImpl               = goccyjson.Marshal
	marshalIndentImpl         = goccyjson.MarshalIndent
	fallbackMarshalImpl       = stdjson.Marshal
	fallbackMarshalIndentImpl = stdjson.MarshalIndent
)

type RawMessage = goccyjson.RawMessage
type Number = goccyjson.Number

func Marshal(v any) ([]byte, error) {
	return marshalWithFallback(
		"json marshal panic",
		func() ([]byte, error) { return marshalImpl(v) },
		func() ([]byte, error) { return fallbackMarshalImpl(v) },
	)
}

func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return marshalWithFallback(
		"json marshal indent panic",
		func() ([]byte, error) { return marshalIndentImpl(v, prefix, indent) },
		func() ([]byte, error) { return fallbackMarshalIndentImpl(v, prefix, indent) },
	)
}

func Unmarshal(data []byte, v any) error {
	return goccyjson.Unmarshal(data, v)
}

func NewDecoder(r io.Reader) *goccyjson.Decoder {
	return goccyjson.NewDecoder(r)
}

func NewEncoder(w io.Writer) *goccyjson.Encoder {
	return goccyjson.NewEncoder(w)
}

func marshalWithFallback(label string, primary func() ([]byte, error), fallback func() ([]byte, error)) ([]byte, error) {
	data, err, recovered := safeMarshalCall(primary)
	if recovered == nil {
		return data, err
	}
	data, err, fallbackRecovered := safeMarshalCall(fallback)
	if fallbackRecovered == nil {
		return data, err
	}
	return nil, fmt.Errorf("%s: primary=%v fallback=%v", label, recovered, fallbackRecovered)
}

func safeMarshalCall(fn func() ([]byte, error)) (data []byte, err error, recovered any) {
	defer func() {
		if r := recover(); r != nil {
			recovered = r
		}
	}()
	data, err = fn()
	return data, err, nil
}
