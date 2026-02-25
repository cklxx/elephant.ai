package llm

import (
	"fmt"
	"io"
)

const maxLLMResponseBodyBytes int64 = 10 * 1024 * 1024

func readResponseBody(reader io.Reader) ([]byte, error) {
	return readLimitedBody(reader, maxLLMResponseBodyBytes)
}

func readLimitedBody(reader io.Reader, limit int64) ([]byte, error) {
	if reader == nil {
		return nil, fmt.Errorf("response body missing")
	}
	if limit <= 0 {
		return io.ReadAll(reader)
	}

	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response body exceeds %d bytes", limit)
	}
	return data, nil
}
