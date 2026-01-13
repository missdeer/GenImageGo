package middleware

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
)

const MaxRecordSize = 5 * 1024 * 1024 // 5MB - reduced from 100MB to prevent OOM

type ResponseRecorder struct {
	http.ResponseWriter
	code        int
	body        *bytes.Buffer
	wroteHeader bool
	size        int
	maxSize     int
	truncated   bool
}

func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{
		ResponseWriter: w,
		code:           http.StatusOK,
		body:           &bytes.Buffer{},
		maxSize:        MaxRecordSize,
	}
}

func (r *ResponseRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.code = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *ResponseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}

	n, err := r.ResponseWriter.Write(b)

	if r.size < r.maxSize {
		remaining := r.maxSize - r.size
		toWrite := n
		if toWrite > remaining {
			toWrite = remaining
			r.truncated = true
		}
		r.body.Write(b[:toWrite])
		r.size += toWrite
	} else {
		r.truncated = true
	}

	return n, err
}

func (r *ResponseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker for websocket support
func (r *ResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		r.truncated = true // Mark as truncated so we don't cache
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

func (r *ResponseRecorder) Code() int {
	return r.code
}

func (r *ResponseRecorder) Body() []byte {
	return r.body.Bytes()
}

func (r *ResponseRecorder) Size() int {
	return r.size
}

func (r *ResponseRecorder) IsTruncated() bool {
	return r.truncated
}

func (r *ResponseRecorder) Headers() http.Header {
	return r.ResponseWriter.Header()
}
