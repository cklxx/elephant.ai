package http

import (
	"compress/gzip"
	"net/http"
	"strings"
)

type gzipResponseWriter struct {
	http.ResponseWriter
	writer      *gzip.Writer
	wroteHeader bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if w.writer != nil {
		_ = w.writer.Flush()
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func acceptsGzip(r *http.Request) bool {
	if r == nil {
		return false
	}
	encoding := r.Header.Get("Accept-Encoding")
	return strings.Contains(strings.ToLower(encoding), "gzip")
}

func shouldSkipCompression(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return true
	}
	path := r.URL.Path
	if isStreamRequest(r) {
		return true
	}
	if strings.HasPrefix(path, "/api/attachments/") || strings.HasPrefix(path, "/api/data/") {
		return true
	}
	return false
}

func CompressionMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldSkipCompression(r) || !acceptsGzip(r) {
				next.ServeHTTP(w, r)
				return
			}

			appendVary(w, "Accept-Encoding")
			w.Header().Set("Content-Encoding", "gzip")

			gz := gzip.NewWriter(w)
			defer gz.Close()

			gzWriter := &gzipResponseWriter{ResponseWriter: w, writer: gz}
			if flusher, ok := w.(http.Flusher); ok {
				gzWriter.ResponseWriter = &responseRecorderFlusher{ResponseWriter: w, Flusher: flusher}
			}
			next.ServeHTTP(gzWriter, r)
		})
	}
}
