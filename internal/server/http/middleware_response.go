package http

import "net/http"

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

// Unwrap exposes the underlying ResponseWriter so downstream handlers
// can recover original capabilities like http.Flusher.
func (r *responseRecorder) Unwrap() http.ResponseWriter {
	if r == nil {
		return nil
	}
	return r.ResponseWriter
}

type responseRecorderFlusher struct {
	http.ResponseWriter
	http.Flusher
}

func (r *responseRecorderFlusher) Unwrap() http.ResponseWriter {
	if r == nil {
		return nil
	}
	return r.ResponseWriter
}

type responseRecorderHijacker struct {
	http.ResponseWriter
	http.Hijacker
}

func (r *responseRecorderHijacker) Unwrap() http.ResponseWriter {
	if r == nil {
		return nil
	}
	return r.ResponseWriter
}

type responseRecorderPusher struct {
	http.ResponseWriter
	http.Pusher
}

func (r *responseRecorderPusher) Unwrap() http.ResponseWriter {
	if r == nil {
		return nil
	}
	return r.ResponseWriter
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	// ResponseRecorder passes through bytes unchanged; handlers are responsible for output encoding.
	n, err := r.ResponseWriter.Write(b)
	if n > 0 {
		r.bytes += int64(n)
	}
	return n, err
}

func wrapResponseWriter(w http.ResponseWriter) (*responseRecorder, http.ResponseWriter) {
	rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
	var wrapped http.ResponseWriter = rec

	if flusher, ok := w.(http.Flusher); ok {
		wrapped = &responseRecorderFlusher{ResponseWriter: wrapped, Flusher: flusher}
	}
	if hijacker, ok := w.(http.Hijacker); ok {
		wrapped = &responseRecorderHijacker{ResponseWriter: wrapped, Hijacker: hijacker}
	}
	if pusher, ok := w.(http.Pusher); ok {
		wrapped = &responseRecorderPusher{ResponseWriter: wrapped, Pusher: pusher}
	}
	return rec, wrapped
}
