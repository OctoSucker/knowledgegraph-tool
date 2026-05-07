package knowledgegraph

import (
	"context"
	"testing"
)

func TestServiceCallAddFactEdgeAndExpand(t *testing.T) {
	t.Parallel()
	store := mustOpenTestStore(t)
	defer store.Close()
	svc, err := NewService(store, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	_, err = svc.Call(context.Background(), ToolAddFactEdge, map[string]any{
		"from_id":       "War Risk",
		"to_id":         "Oil Up",
		"relation_type": "increases_probability_of",
		"confidence":    0.75,
	})
	if err != nil {
		t.Fatalf("add fact edge: %v", err)
	}
	out, err := svc.Call(context.Background(), ToolExpandReasoning, map[string]any{
		"start_id":   "war risk",
		"graph_kind": "knowledge",
		"max_depth":  2,
	})
	if err != nil {
		t.Fatalf("expand reasoning: %v", err)
	}
	hits, ok := out["hits"].([]map[string]any)
	if !ok {
		t.Fatalf("hits has unexpected type: %#v", out["hits"])
	}
	if len(hits) == 0 {
		t.Fatalf("expected non-empty hits")
	}
	steps, ok := hits[0]["steps"].([]map[string]any)
	if !ok {
		t.Fatalf("steps has unexpected type: %#v", hits[0]["steps"])
	}
	if len(steps) == 0 {
		t.Fatalf("expected non-empty steps in first hit")
	}
}
