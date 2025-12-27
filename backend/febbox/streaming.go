// streaming.go - Additional streaming support
package feb_box

import (
    "net/http"
)

// StreamWriter implements http.ResponseWriter for streaming
type StreamWriter struct {
    http.ResponseWriter
    written bool
}

func (sw *StreamWriter) WriteHeader(code int) {
    if !sw.written {
        // Set streaming headers
        sw.ResponseWriter.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
        sw.ResponseWriter.Header().Set("Pragma", "no-cache")
        sw.ResponseWriter.Header().Set("Expires", "0")
        sw.ResponseWriter.Header().Set("Accept-Ranges", "bytes")
        sw.ResponseWriter.Header().Del("Content-Length") // Let it chunk
        
        sw.written = true
    }
    sw.ResponseWriter.WriteHeader(code)
}

func (sw *StreamWriter) Write(data []byte) (int, error) {
    sw.WriteHeader(http.StatusOK)
    return sw.ResponseWriter.Write(data)
}

// StreamServer creates a streaming HTTP server
func StreamServer(handler http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sw := &StreamWriter{ResponseWriter: w}
        handler.ServeHTTP(sw, r)
    })
}

// BufferPool for efficient streaming
type BufferPool struct {
    pool chan []byte
    size int
}

func NewBufferPool(poolSize, bufferSize int) *BufferPool {
    return &BufferPool{
        pool: make(chan []byte, poolSize),
        size: bufferSize,
    }
}

func (bp *BufferPool) Get() []byte {
    select {
    case buf := <-bp.pool:
        return buf
    default:
        return make([]byte, bp.size)
    }
}

func (bp *BufferPool) Put(buf []byte) {
    select {
    case bp.pool <- buf:
    default:
        // Pool is full, discard buffer
    }
}