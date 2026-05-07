package knowledgegraph

import (
	"context"
	"path/filepath"
	"testing"
)

func TestService_Call_addEdgeAndListNodes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := filepath.Join(t.TempDir(), "kg.sqlite")
	store, err := OpenStore(StoreConfig{DBPath: db})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	svc, err := NewService(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	_, err = svc.Call(ctx, ToolAddEdge, map[string]any{
		"from_id": "a",
		"to_id":   "b",
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.Call(ctx, ToolListNodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids, _ := out["node_ids"].([]string)
	if len(ids) != 2 {
		t.Fatalf("node_ids: %#v", out["node_ids"])
	}
}
