package httpclient

import (
	"errors"
	"fmt"
	"io"
)

// ResponseTooLargeError reports that the response body exceeded the limit.
type ResponseTooLargeError struct {
	Limit int64
}

func (e ResponseTooLargeError) Error() string {
	return fmt.Sprintf("response body exceeded limit of %d bytes", e.Limit)
}

// IsResponseTooLarge reports whether the error indicates a response limit violation.
func IsResponseTooLarge(err error) bool {
	var limitErr ResponseTooLargeError
	return errors.As(err, &limitErr)
}

// ReadAllWithLimit reads the response body up to the provided limit.
// If limit <= 0, it behaves like io.ReadAll.
func ReadAllWithLimit(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return io.ReadAll(r)
	}
	lr := &io.LimitedReader{R: r, N: limit + 1}
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ResponseTooLargeError{Limit: limit}
	}
	return data, nil
}
