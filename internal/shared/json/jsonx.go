package jsonx

import "github.com/goccy/go-json"

// Thin wrapper so hot paths can swap JSON implementations in one place.
var (
	Marshal       = json.Marshal
	MarshalIndent = json.MarshalIndent
	Unmarshal     = json.Unmarshal
	NewDecoder    = json.NewDecoder
	NewEncoder    = json.NewEncoder
)

type RawMessage = json.RawMessage
type Number = json.Number
