package redis_test

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
)

func BenchmarkJSONMarshal(b *testing.B) {
	result := &entity.ModerationResult{
		ID:         "result-1",
		ContentID:  "content-1",
		IsApproved: true,
		Score:      0.95,
		ModelName:  "test-model",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(result)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
}

func BenchmarkJSONEncoderWithPool(b *testing.B) {
	result := &entity.ModerationResult{
		ID:         "result-1",
		ContentID:  "content-1",
		IsApproved: true,
		Score:      0.95,
		ModelName:  "test-model",
	}
	var pool = sync.Pool{
		New: func() any { return new(bytes.Buffer) },
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get().(*bytes.Buffer)
		buf.Reset()
		json.NewEncoder(buf).Encode(result)
		_ = buf.Bytes()
		pool.Put(buf)
	}
	b.ReportAllocs()
}
