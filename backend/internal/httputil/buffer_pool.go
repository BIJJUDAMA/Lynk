// Package httputil buffer pooling.
// 100% pure 7-bit ASCII
package httputil

import (
	"bytes"
	"sync"
)

const (
	// InitialBufferSize is the initial capacity for pooled buffers (4 KB).
	InitialBufferSize = 4096

	// MaxPooledBufferCap is the threshold beyond which a buffer is discarded rather than pooled
	// to prevent long-lived heap retention of abnormally large payloads (64 KB).
	MaxPooledBufferCap = 64 * 1024
)

var bufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, InitialBufferSize))
	},
}

// GetBuffer retrieves a recycled buffer from the pool or allocates a new one.
func GetBuffer() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

// PutBuffer resets and returns a buffer to the pool if its capacity is within safe bounds.
func PutBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}

	// Discard buffers that grew beyond safe threshold to prevent memory bloat
	if buf.Cap() > MaxPooledBufferCap {
		return
	}

	buf.Reset()
	bufPool.Put(buf)
}
