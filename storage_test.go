package knowledgegraph

import (
	"path/filepath"
	"testing"
)

func mustOpenTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	store, err := OpenStore(StoreConfig{
		WorkspaceRoot: root,
		DBPath:        filepath.Join(root, "kg.sqlite"),
	})
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	return store
}

func TestStoreEdgeUpsertAndSelect(t *testing.T) {
	t.Parallel()
	s := mustOpenTestStore(t)
	defer s.Close()
	if err := s.NodeUpsert("a", "entity", "[]", "active", nil); err != nil {
		t.Fatalf("upsert node a: %v", err)
	}
	if err := s.NodeUpsert("b", "entity", "[]", "active", nil); err != nil {
		t.Fatalf("upsert node b: %v", err)
	}
	edgeID, err := s.EdgeUpsert(EdgeInput{
		FromID:       "a",
		ToID:         "b",
		GraphKind:    "knowledge",
		RelationType: "causes",
		Polarity:     1,
		Confidence:   0.7,
	})
	if err != nil {
		t.Fatalf("edge upsert: %v", err)
	}
	if edgeID <= 0 {
		t.Fatalf("expected positive edge id")
	}
	rows, err := s.EdgesSelectAll()
	if err != nil {
		t.Fatalf("select edges: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(rows))
	}
}

func TestResolveDBPathRequiresConfig(t *testing.T) {
	t.Parallel()
	if _, err := resolveDBPath(StoreConfig{}); err == nil {
		t.Fatalf("expected resolveDBPath to fail with empty config")
	}
}

func TestOpenStoreDBPathAndRepeatedMigrate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dbPath := filepath.Join(root, "kg.sqlite")

	s1, err := OpenStore(StoreConfig{DBPath: dbPath})
	if err != nil {
		t.Fatalf("open store first time: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	s2, err := OpenStore(StoreConfig{DBPath: dbPath})
	if err != nil {
		t.Fatalf("open store second time: %v", err)
	}
	defer s2.Close()
}
