package knowledgegraph

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
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
	return g.UpsertNode(ctx, NodeUpsert{
		ID:       id,
		NodeType: "entity",
		Status:   "active",
	})
}

type NodeUpsert struct {
	ID       string
	NodeType string
	Aliases  []string
	Status   string
}

func (g *Graph) UpsertNode(ctx context.Context, in NodeUpsert) error {
	_ = ctx
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return fmt.Errorf("knowledgegraph: upsert node: empty id")
	}
	nodeType := strings.TrimSpace(in.NodeType)
	if nodeType == "" {
		nodeType = "entity"
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "active"
	}
	aliasesJSON, err := encodeStringListJSON(in.Aliases)
	if err != nil {
		return fmt.Errorf("knowledgegraph: upsert node aliases: %w", err)
	}
	var blob []byte
	if g.embedder != nil {
		vec, err := g.embedder.Embed(ctx, id)
		if err != nil {
			return fmt.Errorf("knowledgegraph: upsert node embed: %w", err)
		}
		blob, err = EncodeEmbeddingF32(vec)
		if err != nil {
			return err
		}
	}
	if err := g.store.NodeUpsert(id, nodeType, aliasesJSON, status, blob); err != nil {
		return fmt.Errorf("knowledgegraph: upsert node: %w", err)
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
	return g.UpsertNode(ctx, NodeUpsert{ID: id, NodeType: "entity", Status: "active"})
}

type EdgeUpsert struct {
	FromID            string
	ToID              string
	GraphKind         string
	RelationType      string
	Polarity          int
	Confidence        float64
	ConditionText     string
	SourceType        string
	SourceRef         string
	ObservedAt        *time.Time
	ValidFrom         *time.Time
	ValidUntil        *time.Time
	DecayHalfLifeDays int
	ExpiresAt         *time.Time
	IsExecutable      bool
	ActivationRule    string
}

func (g *Graph) UpsertEdge(ctx context.Context, in EdgeUpsert) (int64, error) {
	now := time.Now().UTC()
	observedAt, validFrom, validUntil, err := normalizeEdgeTimes(in.ObservedAt, in.ValidFrom, in.ValidUntil, now)
	if err != nil {
		return 0, err
	}
	if err := g.EnsureNode(ctx, in.FromID); err != nil {
		return 0, err
	}
	if err := g.EnsureNode(ctx, in.ToID); err != nil {
		return 0, err
	}
	id, err := g.store.EdgeUpsert(EdgeInput{
		FromID:            in.FromID,
		ToID:              in.ToID,
		GraphKind:         in.GraphKind,
		RelationType:      in.RelationType,
		Polarity:          in.Polarity,
		Confidence:        in.Confidence,
		ConditionText:     in.ConditionText,
		SourceType:        in.SourceType,
		SourceRef:         in.SourceRef,
		ObservedAt:        observedAt,
		ValidFrom:         validFrom,
		ValidUntil:        validUntil,
		DecayHalfLifeDays: in.DecayHalfLifeDays,
		ExpiresAt:         in.ExpiresAt,
		IsExecutable:      in.IsExecutable,
		ActivationRule:    in.ActivationRule,
	})
	if err != nil {
		return 0, fmt.Errorf("knowledgegraph: upsert edge: %w", err)
	}
	return id, nil
}

type ReasoningHit struct {
	NodeID string   `json:"node_id"`
	Score  float64  `json:"score"`
	Depth  int      `json:"depth"`
	Path   []string `json:"path"`
}

func (g *Graph) ExpandReasoning(startID, graphKind string, asOf time.Time, maxDepth, maxBranch, maxResults int, includeNegative bool, minScore float64) ([]ReasoningHit, error) {
	if strings.TrimSpace(startID) == "" {
		return nil, fmt.Errorf("knowledgegraph: expand reasoning: empty start_id")
	}
	if maxDepth <= 0 {
		maxDepth = 3
	}
	if maxBranch <= 0 {
		maxBranch = 5
	}
	if maxResults <= 0 {
		maxResults = 10
	}
	if minScore < 0 {
		minScore = 0
	}
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	asOf = asOf.UTC()
	rows, err := g.AllEdges()
	if err != nil {
		return nil, err
	}
	adj := make(map[string][]EdgeRow)
	for _, e := range rows {
		if graphKind != "" && e.GraphKind != graphKind {
			continue
		}
		adj[e.FromID] = append(adj[e.FromID], e)
	}
	type state struct {
		node  string
		score float64
		depth int
		path  []string
	}
	best := map[string]ReasoningHit{}
	queue := []state{{node: startID, score: 1, depth: 0, path: []string{startID}}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= maxDepth {
			continue
		}
		out := adj[cur.node]
		sort.Slice(out, func(i, j int) bool {
			return edgeWeight(out[i], asOf) > edgeWeight(out[j], asOf)
		})
		if len(out) > maxBranch {
			out = out[:maxBranch]
		}
		for _, e := range out {
			if !includeNegative && e.Polarity < 0 {
				continue
			}
			w := edgeWeight(e, asOf)
			if w <= 0 {
				continue
			}
			nextScore := cur.score * w
			if nextScore < depthMinScore(minScore, cur.depth+1) {
				continue
			}
			nextPath := append(append([]string{}, cur.path...), e.ToID)
			if h, ok := best[e.ToID]; !ok || nextScore > h.Score {
				best[e.ToID] = ReasoningHit{
					NodeID: e.ToID,
					Score:  nextScore,
					Depth:  cur.depth + 1,
					Path:   nextPath,
				}
			}
			queue = append(queue, state{
				node:  e.ToID,
				score: nextScore,
				depth: cur.depth + 1,
				path:  nextPath,
			})
		}
	}
	hits := make([]ReasoningHit, 0, len(best))
	for _, h := range best {
		hits = append(hits, h)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].NodeID < hits[j].NodeID
		}
		return hits[i].Score > hits[j].Score
	})
	if len(hits) > maxResults {
		hits = hits[:maxResults]
	}
	return hits, nil
}

func edgeWeight(e EdgeRow, asOf time.Time) float64 {
	if e.ValidFrom != nil && asOf.Before(e.ValidFrom.UTC()) {
		return 0
	}
	if e.ValidUntil != nil && asOf.After(e.ValidUntil.UTC()) {
		return 0
	}
	if e.ExpiresAt != nil && asOf.After(e.ExpiresAt.UTC()) {
		return 0
	}
	anchor := e.UpdatedAt
	if e.ObservedAt != nil {
		anchor = e.ObservedAt.UTC()
	}
	days := asOf.Sub(anchor).Hours() / 24
	if days < 0 {
		days = 0
	}
	half := float64(e.DecayHalfLifeDays)
	if half <= 0 {
		half = 30
	}
	freshness := math.Pow(0.5, days/half)
	verify := 1.0 + 0.08*float64(e.EvidenceCount) - 0.12*float64(e.FailedCount)
	if verify < 0.3 {
		verify = 0.3
	}
	if verify > 1.6 {
		verify = 1.6
	}
	score := e.Confidence * freshness * verify
	if e.Polarity < 0 {
		score *= 0.9
	}
	return score
}

func depthMinScore(base float64, depth int) float64 {
	if base <= 0 {
		return 0
	}
	if depth <= 1 {
		return base
	}
	// Increase threshold with depth to suppress long-tail noise in dense graphs.
	// depth=2 -> 1.35x, depth=3 -> 1.70x, depth=4 -> 2.05x ...
	return base * (1 + 0.35*float64(depth-1))
}

func (g *Graph) AttachEdgeEvidence(edgeID int64, sourceType, sourceRef, snippet string, supports bool, weight float64, observedAt *time.Time) error {
	return g.store.EdgeEvidenceInsert(EdgeEvidenceInput{
		EdgeID:     edgeID,
		SourceType: sourceType,
		SourceRef:  sourceRef,
		Snippet:    snippet,
		ObservedAt: observedAt,
		Supports:   supports,
		Weight:     weight,
	})
}

func (g *Graph) VerifyEdge(edgeID int64, success bool, confidence *float64, verifiedAt time.Time) error {
	if verifiedAt.IsZero() {
		verifiedAt = time.Now().UTC()
	}
	return g.store.EdgeVerify(edgeID, success, confidence, verifiedAt)
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
	rows, err := g.store.NodesSelectAll()
	if err != nil {
		return nil, fmt.Errorf("knowledgegraph: list nodes: %w", err)
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	sort.Strings(out)
	return out, nil
}

func (g *Graph) AllNodes() ([]NodeRow, error) {
	rows, err := g.store.NodesSelectAll()
	if err != nil {
		return nil, fmt.Errorf("knowledgegraph: list nodes: %w", err)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ID < rows[j].ID
	})
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

func encodeStringListJSON(items []string) (string, error) {
	if len(items) == 0 {
		return "[]", nil
	}
	clean := make([]string, 0, len(items))
	for _, v := range items {
		s := strings.TrimSpace(v)
		if s == "" {
			continue
		}
		clean = append(clean, s)
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func normalizeEdgeTimes(observedAt, validFrom, validUntil *time.Time, now time.Time) (*time.Time, *time.Time, *time.Time, error) {
	obs := now.UTC()
	if observedAt != nil {
		obs = observedAt.UTC()
	}
	vf := obs
	if validFrom != nil {
		vf = validFrom.UTC()
	}
	var vu *time.Time
	if validUntil != nil {
		t := validUntil.UTC()
		vu = &t
	}
	if vu != nil && vu.Before(vf) {
		return nil, nil, nil, fmt.Errorf("knowledgegraph: upsert edge: valid_until must be >= valid_from")
	}
	obsCopy := obs
	vfCopy := vf
	return &obsCopy, &vfCopy, vu, nil
}
