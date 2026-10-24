package httputil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type samplePayload struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Categories  []string `json:"categories"`
	Active      bool     `json:"active"`
}

var testData = samplePayload{
	ID:          "p-12345678-abcd-ef01-2345-6789abcdef01",
	Title:       "Autonomous Rover Firmware System",
	Description: "Cutting-edge robotics firmware with real-time sensor fusion and path planning.",
	Categories:  []string{"Engineering", "Robotics", "Embedded", "Firmware"},
	Active:      true,
}

func BenchmarkWriteSuccess_Unpooled(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Type", "application/json")
		rec.WriteHeader(http.StatusOK)
		buf := bytes.NewBuffer(nil)
		_ = json.NewEncoder(buf).Encode(Response[samplePayload]{
			Success: true,
			Data:    testData,
		})
		_, _ = rec.Write(buf.Bytes())
	}
}

func BenchmarkWriteSuccess_Pooled(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		WriteSuccess(rec, http.StatusOK, testData)
	}
}

func TestBufferPool_CapGuard(t *testing.T) {
	buf := GetBuffer()
	// Artificially expand buffer beyond MaxPooledBufferCap (64 KB)
	largeData := make([]byte, 100*1024)
	buf.Write(largeData)

	if buf.Cap() <= MaxPooledBufferCap {
		t.Fatalf("expected buffer cap > %d, got %d", MaxPooledBufferCap, buf.Cap())
	}

	// PutBuffer should discard it
	PutBuffer(buf)

	// Next GetBuffer should return a fresh buffer with InitialBufferSize
	fresh := GetBuffer()
	if fresh.Cap() > MaxPooledBufferCap {
		t.Errorf("expected fresh buffer to have cap <= %d, got %d", MaxPooledBufferCap, fresh.Cap())
	}
	PutBuffer(fresh)
}
