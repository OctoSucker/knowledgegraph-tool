package knowledgegraph

import (
	"context"
	"path/filepath"
	"testing"
)

func TestGraph_addNodeAndEdge_withoutEmbedder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := filepath.Join(t.TempDir(), "kg.sqlite")
	store, err := OpenStore(StoreConfig{DBPath: db})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	g, err := NewGraph(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.AddNode(ctx, "alpha"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddNode(ctx, "beta"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddEdge("alpha", "beta", true); err != nil {
		t.Fatal(err)
	}
	ids, err := g.AllNodeIDs()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("nodes: %v", ids)
	}
	edges, err := g.AllEdges()
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 || edges[0].FromID != "alpha" || edges[0].ToID != "beta" || !edges[0].Positive {
		t.Fatalf("edges: %+v", edges)
	}
}
