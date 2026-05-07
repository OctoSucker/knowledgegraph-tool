package knowledgegraph

import (
	"context"
	"fmt"
	"sort"
)

const (
	DefaultEmbeddingMinCosine    = 0.82
	DefaultEmbeddingAmbiguityGap = 0.03
)

type Graph struct {
	store           *Store
	embedder        Embedder
	minCosine       float64
	embeddingMargin float64
}

func NewGraph(store *Store, embedder Embedder) (*Graph, error) {
	if store == nil {
		return nil, fmt.Errorf("knowledgegraph: store is nil")
	}
	return &Graph{
		store:           store,
		embedder:        embedder,
		minCosine:       DefaultEmbeddingMinCosine,
		embeddingMargin: DefaultEmbeddingAmbiguityGap,
	}, nil
}

func (g *Graph) AddNode(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("knowledgegraph: add node: empty id")
	}
	exists, err := g.store.NodeExists(id)
	if err != nil {
		return fmt.Errorf("knowledgegraph: add node: %w", err)
	}
	if exists {
		return fmt.Errorf("knowledgegraph: add node: node %q already exists", id)
	}
	var blob []byte
	if g.embedder != nil {
		vec, err := g.embedder.Embed(ctx, id)
		if err != nil {
			return fmt.Errorf("knowledgegraph: add node embed: %w", err)
		}
		blob, err = EncodeEmbeddingF32(vec)
		if err != nil {
			return err
		}
	}
	if err := g.store.NodeInsert(id, blob); err != nil {
		return fmt.Errorf("knowledgegraph: add node: %w", err)
	}
	return nil
}

func (g *Graph) EnsureNode(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("knowledgegraph: ensure node: empty id")
	}
	exists, err := g.store.NodeExists(id)
	if err != nil {
		return fmt.Errorf("knowledgegraph: ensure node: %w", err)
	}
	if exists {
		return nil
	}
	return g.AddNode(ctx, id)
}

func (g *Graph) AddEdge(fromID, toID string, positive bool) error {
	if fromID == "" || toID == "" {
		return fmt.Errorf("knowledgegraph: add edge: empty from or to")
	}
	fromOK, err := g.store.NodeExists(fromID)
	if err != nil {
		return fmt.Errorf("knowledgegraph: add edge: %w", err)
	}
	if !fromOK {
		return fmt.Errorf("knowledgegraph: add edge: from node %q does not exist", fromID)
	}
	toOK, err := g.store.NodeExists(toID)
	if err != nil {
		return fmt.Errorf("knowledgegraph: add edge: %w", err)
	}
	if !toOK {
		return fmt.Errorf("knowledgegraph: add edge: to node %q does not exist", toID)
	}
	exists, err := g.store.EdgeExists(fromID, toID)
	if err != nil {
		return fmt.Errorf("knowledgegraph: add edge: %w", err)
	}
	if exists {
		return nil
	}
	if err := g.store.EdgeInsert(fromID, toID, positive); err != nil {
		return fmt.Errorf("knowledgegraph: add edge: %w", err)
	}
	return nil
}

func (g *Graph) CanonicalFor(term string) (string, bool, error) {
	if term == "" {
		return "", false, nil
	}
	exists, err := g.store.NodeExists(term)
	if err != nil {
		return "", false, fmt.Errorf("knowledgegraph: canonical exact: %w", err)
	}
	if exists {
		return term, true, nil
	}
	return "", false, nil
}

func (g *Graph) CanonicalForContext(ctx context.Context, term string) (string, bool, error) {
	if term == "" {
		return "", false, nil
	}
	exact, ok, err := g.CanonicalFor(term)
	if err != nil || ok {
		return exact, ok, err
	}
	if g.embedder == nil {
		return "", false, fmt.Errorf("knowledgegraph: semantic lookup requires embedder")
	}
	rows, err := g.embeddingRowsFromStore()
	if err != nil {
		return "", false, err
	}
	if len(rows) == 0 {
		return "", false, nil
	}
	q, err := g.embedder.Embed(ctx, term)
	if err != nil {
		return "", false, err
	}
	id, ok := bestCosineMatch(q, rows, g.minCosine, g.embeddingMargin)
	return id, ok, nil
}

func (g *Graph) AllNodeIDs() ([]string, error) {
	rows, err := g.AllNodes()
	if err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out, nil
}

// AllNodes returns every node row, sorted by id. Embedding is the on-disk BLOB (little-endian float32); when JSON-encoded it appears as a base64 string per encoding/json rules for []byte.
func (g *Graph) AllNodes() ([]NodeRow, error) {
	if g == nil || g.store == nil {
		return nil, fmt.Errorf("knowledgegraph: graph or store is nil")
	}
	rows, err := g.store.NodesSelectAll()
	if err != nil {
		return nil, fmt.Errorf("knowledgegraph: list nodes: %w", err)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}

func (g *Graph) AllEdges() ([]EdgeRow, error) {
	rows, err := g.store.EdgesSelectAll()
	if err != nil {
		return nil, fmt.Errorf("knowledgegraph: list edges: %w", err)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].FromID != rows[j].FromID {
			return rows[i].FromID < rows[j].FromID
		}
		return rows[i].ToID < rows[j].ToID
	})
	return rows, nil
}

type embeddingRow struct {
	id string
	v  []float32
}

func (g *Graph) embeddingRowsFromStore() ([]embeddingRow, error) {
	rows, err := g.store.NodesSelectAll()
	if err != nil {
		return nil, fmt.Errorf("knowledgegraph: list embeddings: %w", err)
	}
	out := make([]embeddingRow, 0, len(rows))
	for _, row := range rows {
		if len(row.Embedding) == 0 {
			continue
		}
		vec, err := DecodeEmbeddingF32(row.Embedding)
		if err != nil {
			return nil, fmt.Errorf("knowledgegraph: decode node %q embedding: %w", row.ID, err)
		}
		cp := make([]float32, len(vec))
		copy(cp, vec)
		out = append(out, embeddingRow{id: row.ID, v: cp})
	}
	return out, nil
}

func bestCosineMatch(q []float32, rows []embeddingRow, minCosine, ambiguityMargin float64) (string, bool) {
	var bestID string
	var best, second float64 = -2, -2
	for _, row := range rows {
		if len(row.v) != len(q) {
			continue
		}
		s := CosineSimilarity(q, row.v)
		if s > best {
			second = best
			best = s
			bestID = row.id
		} else if s > second {
			second = s
		}
	}
	if bestID == "" || best < minCosine {
		return "", false
	}
	if second > best-ambiguityMargin {
		return "", false
	}
	return bestID, true
}
