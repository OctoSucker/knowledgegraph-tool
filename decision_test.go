package knowledgegraph

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRecordDecisionRequiresDisciplineFields(t *testing.T) {
	t.Parallel()
	store := mustOpenTestStore(t)
	defer store.Close()
	svc, err := NewService(store, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	_, err = svc.Call(context.Background(), ToolRecordDecision, map[string]any{
		"market": "Oil market",
		"thesis": "Escalation risk is underpriced",
		"action": "buy",
	})
	if err == nil {
		t.Fatalf("expected missing discipline fields to fail")
	}
	if !strings.Contains(err.Error(), "evidence") {
		t.Fatalf("expected evidence-related error, got %v", err)
	}
}

func TestRecordDecisionWritesEvidenceCounterAndFailureEdges(t *testing.T) {
	t.Parallel()
	store := mustOpenTestStore(t)
	defer store.Close()
	svc, err := NewService(store, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	out, err := svc.Call(context.Background(), ToolRecordDecision, map[string]any{
		"market":             "Oil market",
		"thesis":             "Escalation risk is underpriced",
		"action":             "buy",
		"confidence":         0.72,
		"evidence":           []any{"shipping insurance rising"},
		"counter_evidence":   []any{"front-month oil already bid"},
		"failure_conditions": []any{"confirmed ceasefire"},
		"next_triggers":      []any{"opec emergency statement"},
		"position_rule":      "max 1u until settlement clarity",
	})
	if err != nil {
		t.Fatalf("record decision: %v", err)
	}
	if out["graph_kind"] != DecisionGraphKind {
		t.Fatalf("unexpected graph kind: %#v", out["graph_kind"])
	}
	rows, err := svc.graph.AllEdges()
	if err != nil {
		t.Fatalf("list edges: %v", err)
	}
	relations := map[string]int{}
	for _, row := range rows {
		if row.GraphKind == DecisionGraphKind {
			relations[row.RelationType]++
		}
	}
	for _, relation := range []string{"has_thesis", "supports_action", "supported_by", "contradicted_by", "invalidated_by", "requires_review_on", "constrained_by"} {
		if relations[relation] == 0 {
			t.Fatalf("missing decision relation %q in %#v", relation, relations)
		}
	}
	hits, err := svc.graph.ExpandReasoning("market:oil market", DecisionGraphKind, time.Now().UTC(), 4, 10, 20, true, 0)
	if err != nil {
		t.Fatalf("expand decision graph: %v", err)
	}
	foundFailure := false
	for _, hit := range hits {
		if strings.HasPrefix(hit.NodeID, "failure:") {
			foundFailure = true
		}
	}
	if !foundFailure {
		t.Fatalf("expected decision expansion with negative edges to include failure condition, hits=%#v", hits)
	}
}

func TestReviewDecisionRequiresLessonsForBadOutcomes(t *testing.T) {
	t.Parallel()
	store := mustOpenTestStore(t)
	defer store.Close()
	svc, err := NewService(store, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	_, err = svc.Call(context.Background(), ToolReviewDecision, map[string]any{
		"market":          "Oil market",
		"thesis":          "Escalation risk is underpriced",
		"outcome":         "incorrect",
		"realized_result": "oil sold off after ceasefire",
	})
	if err == nil {
		t.Fatalf("expected missing lessons to fail for incorrect outcome")
	}
	if !strings.Contains(err.Error(), "lessons") {
		t.Fatalf("expected lessons error, got %v", err)
	}
}

func TestReviewDecisionWritesReviewAndFailsThesisActionEdge(t *testing.T) {
	t.Parallel()
	store := mustOpenTestStore(t)
	defer store.Close()
	svc, err := NewService(store, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	_, err = svc.Call(context.Background(), ToolRecordDecision, map[string]any{
		"market":             "Oil market",
		"thesis":             "Escalation risk is underpriced",
		"action":             "buy",
		"confidence":         0.72,
		"evidence":           []any{"shipping insurance rising"},
		"counter_evidence":   []any{"front-month oil already bid"},
		"failure_conditions": []any{"confirmed ceasefire"},
	})
	if err != nil {
		t.Fatalf("record decision: %v", err)
	}
	out, err := svc.Call(context.Background(), ToolReviewDecision, map[string]any{
		"market":          "Oil market",
		"thesis":          "Escalation risk is underpriced",
		"outcome":         "incorrect",
		"realized_result": "ceasefire confirmed and oil sold off",
		"lessons":         []any{"do not ignore ceasefire verification path"},
		"rule_updates":    []any{"reduce size when failure condition is imminent"},
	})
	if err != nil {
		t.Fatalf("review decision: %v", err)
	}
	verified, ok := out["verified_edges"].([]int64)
	if !ok || len(verified) == 0 {
		t.Fatalf("expected failed verified edges, got %#v", out["verified_edges"])
	}
	rows, err := svc.graph.AllEdges()
	if err != nil {
		t.Fatalf("list edges: %v", err)
	}
	foundReview := false
	foundFailedAction := false
	for _, row := range rows {
		if row.RelationType == "produced_lesson" || row.RelationType == "updates_rule" {
			foundReview = true
		}
		if row.RelationType == "supports_action" && row.FailedCount > 0 {
			foundFailedAction = true
		}
	}
	if !foundReview {
		t.Fatalf("expected review lesson/rule edges")
	}
	if !foundFailedAction {
		t.Fatalf("expected supports_action edge to be marked failed")
	}
}
