package jsonx

import (
	stdjson "encoding/json"
	"fmt"

	goccyjson "github.com/goccy/go-json"
)

// Thin wrapper so hot paths can swap JSON implementations in one place.
var (
	marshalImpl               = goccyjson.Marshal
	marshalIndentImpl         = goccyjson.MarshalIndent
	fallbackMarshalImpl       = stdjson.Marshal
	fallbackMarshalIndentImpl = stdjson.MarshalIndent

	Marshal       = marshal
	MarshalIndent = marshalIndent
	Unmarshal     = goccyjson.Unmarshal
	NewDecoder    = goccyjson.NewDecoder
	NewEncoder    = goccyjson.NewEncoder
)

type RawMessage = goccyjson.RawMessage
type Number = goccyjson.Number

func marshal(v any) ([]byte, error) {
	data, err, recovered := safeMarshalCall(func() ([]byte, error) {
		return marshalImpl(v)
	})
	if recovered == nil {
		return data, err
	}
	data, err, fallbackRecovered := safeMarshalCall(func() ([]byte, error) {
		return fallbackMarshalImpl(v)
	})
	if fallbackRecovered == nil {
		return data, err
	}
	return nil, fmt.Errorf("json marshal panic: primary=%v fallback=%v", recovered, fallbackRecovered)
}

func marshalIndent(v any, prefix, indent string) ([]byte, error) {
	data, err, recovered := safeMarshalCall(func() ([]byte, error) {
		return marshalIndentImpl(v, prefix, indent)
	})
	if recovered == nil {
		return data, err
	}
	data, err, fallbackRecovered := safeMarshalCall(func() ([]byte, error) {
		return fallbackMarshalIndentImpl(v, prefix, indent)
	})
	if fallbackRecovered == nil {
		return data, err
	}
	return nil, fmt.Errorf("json marshal indent panic: primary=%v fallback=%v", recovered, fallbackRecovered)
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
