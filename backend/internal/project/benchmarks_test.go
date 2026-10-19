package project

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func BenchmarkCursor_EncodeDecode(b *testing.B) {
	t := time.Now().UTC()
	id := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cur := EncodeCursor(t, id)
		_, _, _ = DecodeCursor(cur)
	}
}
