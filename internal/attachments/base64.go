package attachments

import "encoding/base64"

// DecodeBase64 attempts standard and raw base64 decoding.
func DecodeBase64(value string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(value)
}
