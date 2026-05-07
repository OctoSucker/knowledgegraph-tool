package knowledgegraph

import (
	"math"
	"testing"
)

func TestEncodeDecodeEmbeddingF32_roundTrip(t *testing.T) {
	t.Parallel()
	orig := []float32{1, 0, -1, 0.5}
	b, err := EncodeEmbeddingF32(orig)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeEmbeddingF32(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(orig) {
		t.Fatalf("len %d want %d", len(got), len(orig))
	}
	for i := range orig {
		if math.Abs(float64(got[i]-orig[i])) > 1e-6 {
			t.Fatalf("idx %d: got %v want %v", i, got[i], orig[i])
		}
	}
}

func TestEncodeEmbeddingF32_empty(t *testing.T) {
	t.Parallel()
	_, err := EncodeEmbeddingF32(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = EncodeEmbeddingF32([]float32{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeEmbeddingF32_badLength(t *testing.T) {
	t.Parallel()
	_, err := DecodeEmbeddingF32([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCosineSimilarity(t *testing.T) {
	t.Parallel()
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	if got := CosineSimilarity(a, b); math.Abs(got-1) > 1e-9 {
		t.Fatalf("same dir: got %v", got)
	}
	c := []float32{0, 1, 0}
	if got := CosineSimilarity(a, c); math.Abs(got) > 1e-9 {
		t.Fatalf("orthogonal: got %v want 0", got)
	}
	if got := CosineSimilarity(a, []float32{1, 0}); got != -1 {
		t.Fatalf("length mismatch: got %v", got)
	}
	if got := CosineSimilarity(nil, nil); got != -1 {
		t.Fatalf("empty: got %v", got)
	}
}
