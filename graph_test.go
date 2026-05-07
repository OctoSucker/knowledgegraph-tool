package knowledgegraph

import (
	"context"
	"testing"
	"time"
)

func TestCanonicalizeNodeIDOnUpsertAndLookup(t *testing.T) {
	t.Parallel()
	s := mustOpenTestStore(t)
	defer s.Close()
	g, err := NewGraph(s, nil)
	if err != nil {
		t.Fatalf("new graph: %v", err)
	}
	if err := g.UpsertNode(context.Background(), NodeUpsert{ID: "  War   Risk  ", NodeType: "event", Status: "active"}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	id, ok, err := g.CanonicalFor("war risk")
	if err != nil {
		t.Fatalf("canonical lookup: %v", err)
	}
	if !ok || id != "war risk" {
		t.Fatalf("expected canonical id %q, got %q (ok=%v)", "war risk", id, ok)
	}
}

func TestExpandReasoningReturnsStepDetailsAndAvoidsCycles(t *testing.T) {
	t.Parallel()
	s := mustOpenTestStore(t)
	defer s.Close()
	g, err := NewGraph(s, nil)
	if err != nil {
		t.Fatalf("new graph: %v", err)
	}
	ctx := context.Background()
	if _, err := g.UpsertEdge(ctx, EdgeUpsert{FromID: "a", ToID: "b", GraphKind: "knowledge", RelationType: "causes", Confidence: 0.9, Polarity: 1}); err != nil {
		t.Fatalf("upsert edge a->b: %v", err)
	}
	if _, err := g.UpsertEdge(ctx, EdgeUpsert{FromID: "b", ToID: "a", GraphKind: "knowledge", RelationType: "feeds_back", Confidence: 0.9, Polarity: 1}); err != nil {
		t.Fatalf("upsert edge b->a: %v", err)
	}

	hits, err := g.ExpandReasoning("a", "knowledge", time.Now().UTC(), 3, 5, 10, false, 0)
	if err != nil {
		t.Fatalf("expand reasoning: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected non-empty hits")
	}
	if len(hits[0].Steps) == 0 {
		t.Fatalf("expected step details in reasoning hit")
	}
	for _, h := range hits {
		if h.NodeID == "a" {
			t.Fatalf("cycle should not re-introduce start node in hits: %+v", h)
		}
		seen := map[string]struct{}{}
		for _, p := range h.Path {
			if _, ok := seen[p]; ok {
				t.Fatalf("path contains duplicate node %q: %+v", p, h.Path)
			}
			seen[p] = struct{}{}
		}
	}
}

func TestExpandReasoningCanonicalizesStartID(t *testing.T) {
	t.Parallel()
	s := mustOpenTestStore(t)
	defer s.Close()
	g, err := NewGraph(s, nil)
	if err != nil {
		t.Fatalf("new graph: %v", err)
	}
	ctx := context.Background()
	if _, err := g.UpsertEdge(ctx, EdgeUpsert{
		FromID:       "war risk",
		ToID:         "oil up",
		GraphKind:    "knowledge",
		RelationType: "increases_probability_of",
		Confidence:   0.8,
		Polarity:     1,
	}); err != nil {
		t.Fatalf("upsert edge: %v", err)
	}
	hits, err := g.ExpandReasoning("War   Risk", "knowledge", time.Now().UTC(), 2, 5, 10, false, 0)
	if err != nil {
		t.Fatalf("expand reasoning: %v", err)
	}
	found := false
	for _, h := range hits {
		if h.NodeID == "oil up" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected hit node %q, got %+v", "oil up", hits)
	}
}

func TestEdgeWeightTemporalAndEvidenceBehavior(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()

	futureFrom := now.Add(24 * time.Hour)
	if w, _ := edgeWeightWithDetail(EdgeRow{
		Confidence:        0.8,
		DecayHalfLifeDays: 30,
		UpdatedAt:         now,
		ValidFrom:         &futureFrom,
	}, now); w != 0 {
		t.Fatalf("expected zero weight when valid_from in future, got %v", w)
	}

	expiredUntil := now.Add(-1 * time.Hour)
	if w, _ := edgeWeightWithDetail(EdgeRow{
		Confidence:        0.8,
		DecayHalfLifeDays: 30,
		UpdatedAt:         now,
		ValidUntil:        &expiredUntil,
	}, now); w != 0 {
		t.Fatalf("expected zero weight when valid_until expired, got %v", w)
	}

	base, _ := edgeWeightWithDetail(EdgeRow{
		Confidence:        0.8,
		DecayHalfLifeDays: 30,
		UpdatedAt:         now,
	}, now)
	moreEvidence, _ := edgeWeightWithDetail(EdgeRow{
		Confidence:        0.8,
		EvidenceCount:     3,
		DecayHalfLifeDays: 30,
		UpdatedAt:         now,
	}, now)
	moreFailures, _ := edgeWeightWithDetail(EdgeRow{
		Confidence:        0.8,
		FailedCount:       3,
		DecayHalfLifeDays: 30,
		UpdatedAt:         now,
	}, now)
	if moreEvidence <= base {
		t.Fatalf("expected evidence to increase score: evidence=%v base=%v", moreEvidence, base)
	}
	if moreFailures >= base {
		t.Fatalf("expected failures to decrease score: failures=%v base=%v", moreFailures, base)
	}
}
