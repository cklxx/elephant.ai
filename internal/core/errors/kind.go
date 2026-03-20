package errors

// ErrorKind classifies errors for routing and handling.
type ErrorKind int

const (
	Unknown      ErrorKind = iota
	InvalidInput
	Config
	Provider
	Tool
	Temporary
	NotFound
)

var kindNames = [...]string{
	Unknown:      "Unknown",
	InvalidInput: "InvalidInput",
	Config:       "Config",
	Provider:     "Provider",
	Tool:         "Tool",
	Temporary:    "Temporary",
	NotFound:     "NotFound",
}

// String returns the kind name.
func (k ErrorKind) String() string {
	if int(k) >= 0 && int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "Unknown"
}
