package knowledgegraph

import "testing"

func TestEmbeddingEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()
	in := []float32{0.1, -2.5, 3.0}
	blob, err := EncodeEmbeddingF32(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := DecodeEmbeddingF32(blob)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("length mismatch: got=%d want=%d", len(out), len(in))
	}
	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("value mismatch at %d: got=%v want=%v", i, out[i], in[i])
		}
	}
}

func TestCosineSimilarityBasic(t *testing.T) {
	t.Parallel()
	if got := CosineSimilarity([]float32{1, 0}, []float32{1, 0}); got < 0.999 {
		t.Fatalf("expected near 1, got %v", got)
	}
	if got := CosineSimilarity([]float32{1, 0}, []float32{0, 1}); got > 0.001 {
		t.Fatalf("expected near 0, got %v", got)
	}
	if got := CosineSimilarity([]float32{0, 0}, []float32{1, 0}); got != -1 {
		t.Fatalf("expected -1 for zero vector, got %v", got)
	}
}

