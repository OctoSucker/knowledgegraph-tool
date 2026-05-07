package knowledgegraph

import (
	"path/filepath"
	"testing"
)

func TestResolveDBPath_requiresInput(t *testing.T) {
	t.Parallel()
	_, err := resolveDBPath(StoreConfig{})
	if err == nil {
		t.Fatal("expected error when workspace and db path are empty")
	}
}

func TestOpenStore_withDBPath(t *testing.T) {
	t.Parallel()
	db := filepath.Join(t.TempDir(), "kg.sqlite")
	store, err := OpenStore(StoreConfig{DBPath: db})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ok, err := store.NodeExists("nope")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("unexpected node")
	}
}

func TestOpenStore_idempotentMigrations(t *testing.T) {
	t.Parallel()
	db := filepath.Join(t.TempDir(), "kg.sqlite")
	s1, err := OpenStore(StoreConfig{DBPath: db})
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := OpenStore(StoreConfig{DBPath: db})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s2.Close() })
}
