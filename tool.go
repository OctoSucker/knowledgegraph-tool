package knowledgegraph

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	ToolUpsertNode         = "kg_upsert_node"
	ToolAddFactEdge        = "kg_add_fact_edge"
	ToolAddSkillEdge       = "kg_add_skill_edge"
	ToolIngestStatement    = "kg_ingest_statement"
	ToolAttachEdgeEvidence = "kg_attach_edge_evidence"
	ToolVerifyEdge         = "kg_verify_edge"
	ToolLookupNodeExact    = "kg_lookup_node_exact"
	ToolLookupNodeSemantic = "kg_lookup_node_semantic"
	ToolListNodes          = "kg_list_nodes"
	ToolListEdges          = "kg_list_edges"
	ToolExpandReasoning    = "kg_expand_reasoning"
)

type Service struct {
	store *Store
	graph *Graph
}

// Service is the single execution entrypoint for external callers.
// Keep CLI and MCP integrations strictly routed through Service.Call.
func NewService(store *Store, embedder Embedder) (*Service, error) {
	graph, err := NewGraph(store, embedder)
	if err != nil {
		return nil, err
	}
	return &Service{store: store, graph: graph}, nil
}

func (s *Service) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Close()
}

func (s *Service) ToolNames() []string {
	return []string{
		ToolUpsertNode,
		ToolAddFactEdge,
		ToolAddSkillEdge,
		ToolIngestStatement,
		ToolAttachEdgeEvidence,
		ToolVerifyEdge,
		ToolLookupNodeExact,
		ToolLookupNodeSemantic,
		ToolListNodes,
		ToolListEdges,
		ToolExpandReasoning,
	}
}

func (s *Service) Call(ctx context.Context, tool string, arguments map[string]any) (map[string]any, error) {
	switch strings.TrimSpace(tool) {
	case ToolUpsertNode:
		id, err := parseRequiredString(arguments, tool, "id")
		if err != nil {
			return nil, err
		}
		nodeType, _ := parseOptionalString(arguments, "node_type", "entity")
		status, _ := parseOptionalString(arguments, "status", "active")
		aliases, err := parseOptionalStringList(arguments, "aliases")
		if err != nil {
			return nil, fmt.Errorf("%s: %w", tool, err)
		}
		if err := s.graph.UpsertNode(ctx, NodeUpsert{ID: id, NodeType: nodeType, Aliases: aliases, Status: status}); err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "node_type": nodeType, "status": status, "aliases": aliases}, nil
	case ToolAddFactEdge:
		in, err := parseEdgeInput(arguments, tool, "knowledge")
		if err != nil {
			return nil, err
		}
		in.IsExecutable = false
		in.ActivationRule = ""
		edgeID, err := s.graph.UpsertEdge(ctx, in)
		if err != nil {
			return nil, err
		}
		return map[string]any{"edge_id": edgeID, "graph_kind": in.GraphKind}, nil
	case ToolAddSkillEdge:
		in, err := parseEdgeInput(arguments, tool, "skill")
		if err != nil {
			return nil, err
		}
		in.IsExecutable = true
		activationRule, _ := parseOptionalString(arguments, "activation_rule", "")
		in.ActivationRule = activationRule
		edgeID, err := s.graph.UpsertEdge(ctx, in)
		if err != nil {
			return nil, err
		}
		return map[string]any{"edge_id": edgeID, "graph_kind": in.GraphKind}, nil
	case ToolIngestStatement:
		statement, err := parseRequiredString(arguments, tool, "statement")
		if err != nil {
			return nil, err
		}
		graphKind, _ := parseOptionalString(arguments, "graph_kind", "knowledge")
		sourceType, _ := parseOptionalString(arguments, "source_type", "llm_extracted")
		sourceRef, _ := parseOptionalString(arguments, "source_ref", "")
		model, _ := parseOptionalString(arguments, "model", "")
		defaultConfidence, _ := parseOptionalFloat(arguments, "default_confidence", DefaultIngestConfidence)
		return s.IngestStatement(ctx, statement, graphKind, sourceType, sourceRef, model, defaultConfidence)
	case ToolExpandReasoning:
		startID, err := parseRequiredString(arguments, tool, "start_id")
		if err != nil {
			return nil, err
		}
		graphKind, _ := parseOptionalString(arguments, "graph_kind", "knowledge")
		maxDepth, _ := parseOptionalInt(arguments, "max_depth", 3)
		maxBranch, _ := parseOptionalInt(arguments, "max_branch", 5)
		maxResults, _ := parseOptionalInt(arguments, "max_results", 10)
		includeNegative, _ := parseOptionalBool(arguments, "include_negative", false)
		minScore, _ := parseOptionalFloat(arguments, "min_score", 0)
		asOf := time.Now().UTC()
		if at, ok, err := parseOptionalTime(arguments, "as_of"); err != nil {
			return nil, err
		} else if ok {
			asOf = at
		}
		hits, err := s.graph.ExpandReasoning(startID, graphKind, asOf, maxDepth, maxBranch, maxResults, includeNegative, minScore)
		if err != nil {
			return nil, err
		}
		out := make([]map[string]any, len(hits))
		for i, h := range hits {
			steps := make([]map[string]any, len(h.Steps))
			for j, st := range h.Steps {
				steps[j] = map[string]any{
					"edge_id":             st.EdgeID,
					"from_id":             st.FromID,
					"to_id":               st.ToID,
					"relation_type":       st.RelationType,
					"raw_confidence":      st.RawConfidence,
					"freshness_factor":    st.FreshnessFactor,
					"verification_factor": st.VerificationFactor,
					"final_weight":        st.FinalWeight,
				}
			}
			out[i] = map[string]any{
				"node_id": h.NodeID,
				"score":   h.Score,
				"depth":   h.Depth,
				"path":    h.Path,
				"steps":   steps,
			}
		}
		return map[string]any{
			"start_id":         startID,
			"as_of":            asOf.Format(time.RFC3339),
			"include_negative": includeNegative,
			"min_score":        minScore,
			"hits":             out,
		}, nil
	case ToolAttachEdgeEvidence:
		edgeID, err := parseRequiredInt64(arguments, tool, "edge_id")
		if err != nil {
			return nil, err
		}
		sourceType, _ := parseOptionalString(arguments, "source_type", "")
		sourceRef, _ := parseOptionalString(arguments, "source_ref", "")
		snippet, _ := parseOptionalString(arguments, "snippet", "")
		supports, err := parseRequiredBool(arguments, tool, "supports")
		if err != nil {
			return nil, err
		}
		weight, _ := parseOptionalFloat(arguments, "weight", 1.0)
		var observedAt *time.Time
		if at, ok, err := parseOptionalTime(arguments, "observed_at"); err != nil {
			return nil, fmt.Errorf("%s: %w", tool, err)
		} else if ok {
			observedAt = &at
		}
		if err := s.graph.AttachEdgeEvidence(edgeID, sourceType, sourceRef, snippet, supports, weight, observedAt); err != nil {
			return nil, err
		}
		return map[string]any{"edge_id": edgeID, "supports": supports, "weight": weight}, nil
	case ToolVerifyEdge:
		edgeID, err := parseRequiredInt64(arguments, tool, "edge_id")
		if err != nil {
			return nil, err
		}
		success, err := parseRequiredBool(arguments, tool, "success")
		if err != nil {
			return nil, err
		}
		var confidence *float64
		if c, ok, err := parseOptionalFloatMaybe(arguments, "confidence"); err != nil {
			return nil, err
		} else if ok {
			confidence = &c
		}
		verifiedAt := time.Now().UTC()
		if at, ok, err := parseOptionalTime(arguments, "verified_at"); err != nil {
			return nil, err
		} else if ok {
			verifiedAt = at
		}
		if err := s.graph.VerifyEdge(edgeID, success, confidence, verifiedAt); err != nil {
			return nil, err
		}
		out := map[string]any{"edge_id": edgeID, "success": success, "verified_at": verifiedAt.Format(time.RFC3339)}
		if confidence != nil {
			out["confidence"] = *confidence
		}
		return out, nil
	case ToolLookupNodeExact:
		term, err := parseRequiredString(arguments, tool, "term")
		if err != nil {
			return nil, err
		}
		canon, ok, err := s.graph.CanonicalFor(term)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("%s: no exact match for %q", tool, term)
		}
		return map[string]any{"canonical": canon, "matched": true}, nil
	case ToolLookupNodeSemantic:
		term, err := parseRequiredString(arguments, tool, "term")
		if err != nil {
			return nil, err
		}
		canon, ok, err := s.graph.CanonicalForContext(ctx, term)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("%s: no match for %q", tool, term)
		}
		return map[string]any{"canonical": canon, "matched": true}, nil
	case ToolListNodes:
		rows, err := s.graph.AllNodes()
		if err != nil {
			return nil, err
		}
		nodes := make([]map[string]any, len(rows))
		for i, n := range rows {
			nodes[i] = map[string]any{
				"id":           n.ID,
				"node_type":    n.NodeType,
				"aliases_json": n.AliasesJSON,
				"status":       n.Status,
				"updated_at":   n.UpdatedAt.Format(time.RFC3339),
			}
		}
		return map[string]any{"nodes": nodes}, nil
	case ToolListEdges:
		rows, err := s.graph.AllEdges()
		if err != nil {
			return nil, err
		}
		edges := make([]map[string]any, len(rows))
		for i, e := range rows {
			item := map[string]any{
				"id":                   e.ID,
				"from_id":              e.FromID,
				"to_id":                e.ToID,
				"graph_kind":           e.GraphKind,
				"relation_type":        e.RelationType,
				"polarity":             e.Polarity,
				"confidence":           e.Confidence,
				"condition_text":       e.ConditionText,
				"source_type":          e.SourceType,
				"source_ref":           e.SourceRef,
				"created_at":           e.CreatedAt.Format(time.RFC3339),
				"evidence_count":       e.EvidenceCount,
				"failed_count":         e.FailedCount,
				"decay_half_life_days": e.DecayHalfLifeDays,
				"is_executable":        e.IsExecutable,
				"activation_rule":      e.ActivationRule,
				"updated_at":           e.UpdatedAt.Format(time.RFC3339),
			}
			if e.LastVerifiedAt != nil {
				item["last_verified_at"] = e.LastVerifiedAt.Format(time.RFC3339)
			}
			if e.ObservedAt != nil {
				item["observed_at"] = e.ObservedAt.Format(time.RFC3339)
			}
			if e.ValidFrom != nil {
				item["valid_from"] = e.ValidFrom.Format(time.RFC3339)
			}
			if e.ValidUntil != nil {
				item["valid_until"] = e.ValidUntil.Format(time.RFC3339)
			}
			if e.ExpiresAt != nil {
				item["expires_at"] = e.ExpiresAt.Format(time.RFC3339)
			}
			edges[i] = item
		}
		return map[string]any{"edges": edges}, nil
	default:
		return nil, fmt.Errorf("knowledgegraph: unknown tool %q", tool)
	}
}

func ToolSchema(tool string) map[string]any {
	switch tool {
	case ToolUpsertNode:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string"},
				"node_type": map[string]any{"type": "string"},
				"aliases":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"status":    map[string]any{"type": "string"},
			},
			"required":             []string{"id"},
			"additionalProperties": false,
		}
	case ToolAddFactEdge:
		return edgeToolSchema(false)
	case ToolAddSkillEdge:
		s := edgeToolSchema(true)
		s["required"] = []string{"from_id", "to_id", "relation_type", "confidence", "activation_rule"}
		return s
	case ToolIngestStatement:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"statement":          map[string]any{"type": "string", "description": "Natural-language conclusion or notes to ingest"},
				"graph_kind":         map[string]any{"type": "string"},
				"source_type":        map[string]any{"type": "string"},
				"source_ref":         map[string]any{"type": "string"},
				"model":              map[string]any{"type": "string", "description": "LLM model for extraction"},
				"default_confidence": map[string]any{"type": "number"},
			},
			"required":             []string{"statement"},
			"additionalProperties": false,
		}
	case ToolAttachEdgeEvidence:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"edge_id":     map[string]any{"type": "integer"},
				"source_type": map[string]any{"type": "string"},
				"source_ref":  map[string]any{"type": "string"},
				"snippet":     map[string]any{"type": "string"},
				"supports":    map[string]any{"type": "boolean"},
				"weight":      map[string]any{"type": "number"},
				"observed_at": map[string]any{"type": "string", "description": "RFC3339"},
			},
			"required":             []string{"edge_id", "supports"},
			"additionalProperties": false,
		}
	case ToolVerifyEdge:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"edge_id":     map[string]any{"type": "integer"},
				"success":     map[string]any{"type": "boolean"},
				"confidence":  map[string]any{"type": "number"},
				"verified_at": map[string]any{"type": "string", "description": "RFC3339"},
			},
			"required":             []string{"edge_id", "success"},
			"additionalProperties": false,
		}
	case ToolLookupNodeExact, ToolLookupNodeSemantic:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"term": map[string]any{"type": "string"},
			},
			"required":             []string{"term"},
			"additionalProperties": false,
		}
	case ToolListNodes, ToolListEdges:
		return map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		}
	case ToolExpandReasoning:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"start_id":         map[string]any{"type": "string"},
				"graph_kind":       map[string]any{"type": "string"},
				"as_of":            map[string]any{"type": "string", "description": "RFC3339"},
				"max_depth":        map[string]any{"type": "integer"},
				"max_branch":       map[string]any{"type": "integer"},
				"max_results":      map[string]any{"type": "integer"},
				"include_negative": map[string]any{"type": "boolean"},
				"min_score":        map[string]any{"type": "number"},
			},
			"required":             []string{"start_id"},
			"additionalProperties": false,
		}
	default:
		return map[string]any{"type": "object"}
	}
}

func ToolDescription(tool string) string {
	switch tool {
	case ToolUpsertNode:
		return "Create or update a typed node with aliases and status."
	case ToolAddFactEdge:
		return "Create or update a knowledge edge with condition, confidence, source, and decay metadata."
	case ToolAddSkillEdge:
		return "Create or update a skill/procedure edge with activation rule."
	case ToolIngestStatement:
		return "Ingest natural-language statement by extracting nodes/edges with LLM and writing them to graph."
	case ToolAttachEdgeEvidence:
		return "Attach an evidence item to an edge and update success/failure counters."
	case ToolVerifyEdge:
		return "Record verification outcome for an edge and optionally update confidence."
	case ToolLookupNodeExact:
		return "Find a stored node id by exact string match."
	case ToolLookupNodeSemantic:
		return "Find a node id by exact-or-semantic lookup."
	case ToolListNodes:
		return "List all graph nodes with metadata."
	case ToolListEdges:
		return "List all graph edges with memory metadata."
	case ToolExpandReasoning:
		return "Expand from a start node using temporal validity and weighted scoring."
	default:
		return ""
	}
}

func edgeToolSchema(withActivation bool) map[string]any {
	props := map[string]any{
		"from_id":              map[string]any{"type": "string"},
		"to_id":                map[string]any{"type": "string"},
		"relation_type":        map[string]any{"type": "string"},
		"polarity":             map[string]any{"type": "integer", "description": "-1/0/1"},
		"confidence":           map[string]any{"type": "number"},
		"condition_text":       map[string]any{"type": "string"},
		"source_type":          map[string]any{"type": "string"},
		"source_ref":           map[string]any{"type": "string"},
		"observed_at":          map[string]any{"type": "string", "description": "RFC3339"},
		"valid_from":           map[string]any{"type": "string", "description": "RFC3339"},
		"valid_until":          map[string]any{"type": "string", "description": "RFC3339"},
		"decay_half_life_days": map[string]any{"type": "integer"},
		"expires_at":           map[string]any{"type": "string", "description": "RFC3339"},
	}
	if withActivation {
		props["activation_rule"] = map[string]any{"type": "string"}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             []string{"from_id", "to_id", "relation_type", "confidence"},
		"additionalProperties": false,
	}
}

func parseEdgeInput(args map[string]any, tool, graphKind string) (EdgeUpsert, error) {
	fromID, err := parseRequiredString(args, tool, "from_id")
	if err != nil {
		return EdgeUpsert{}, err
	}
	toID, err := parseRequiredString(args, tool, "to_id")
	if err != nil {
		return EdgeUpsert{}, err
	}
	relationType, err := parseRequiredString(args, tool, "relation_type")
	if err != nil {
		return EdgeUpsert{}, err
	}
	confidence, err := parseRequiredFloat(args, tool, "confidence")
	if err != nil {
		return EdgeUpsert{}, err
	}
	polarity, _ := parseOptionalInt(args, "polarity", 1)
	conditionText, _ := parseOptionalString(args, "condition_text", "")
	sourceType, _ := parseOptionalString(args, "source_type", "")
	sourceRef, _ := parseOptionalString(args, "source_ref", "")
	var observedAt *time.Time
	if at, ok, err := parseOptionalTime(args, "observed_at"); err != nil {
		return EdgeUpsert{}, err
	} else if ok {
		observedAt = &at
	}
	var validFrom *time.Time
	if at, ok, err := parseOptionalTime(args, "valid_from"); err != nil {
		return EdgeUpsert{}, err
	} else if ok {
		validFrom = &at
	}
	var validUntil *time.Time
	if at, ok, err := parseOptionalTime(args, "valid_until"); err != nil {
		return EdgeUpsert{}, err
	} else if ok {
		validUntil = &at
	}
	decayHalfLifeDays, _ := parseOptionalInt(args, "decay_half_life_days", 30)
	var expiresAt *time.Time
	if at, ok, err := parseOptionalTime(args, "expires_at"); err != nil {
		return EdgeUpsert{}, err
	} else if ok {
		expiresAt = &at
	}
	return EdgeUpsert{
		FromID:            fromID,
		ToID:              toID,
		GraphKind:         graphKind,
		RelationType:      relationType,
		Polarity:          polarity,
		Confidence:        confidence,
		ConditionText:     conditionText,
		SourceType:        sourceType,
		SourceRef:         sourceRef,
		ObservedAt:        observedAt,
		ValidFrom:         validFrom,
		ValidUntil:        validUntil,
		DecayHalfLifeDays: decayHalfLifeDays,
		ExpiresAt:         expiresAt,
	}, nil
}

func parseRequiredString(args map[string]any, tool, field string) (string, error) {
	if args == nil {
		return "", fmt.Errorf("%s: arguments required", tool)
	}
	raw, ok := args[field]
	if !ok {
		return "", fmt.Errorf("%s: %s is required", tool, field)
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s: %s must be a string", tool, field)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("%s: %s must be non-empty", tool, field)
	}
	return s, nil
}

func parseOptionalString(args map[string]any, field, fallback string) (string, error) {
	if args == nil {
		return fallback, nil
	}
	raw, ok := args[field]
	if !ok || raw == nil {
		return fallback, nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", field)
	}
	return strings.TrimSpace(s), nil
}

func parseOptionalStringList(args map[string]any, field string) ([]string, error) {
	if args == nil {
		return nil, nil
	}
	raw, ok := args[field]
	if !ok || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array of strings", field)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be an array of strings", field)
		}
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

func parseRequiredBool(args map[string]any, tool, field string) (bool, error) {
	if args == nil {
		return false, fmt.Errorf("%s: arguments required", tool)
	}
	raw, ok := args[field]
	if !ok {
		return false, fmt.Errorf("%s: %s is required", tool, field)
	}
	v, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("%s: %s must be a boolean", tool, field)
	}
	return v, nil
}

func parseRequiredInt64(args map[string]any, tool, field string) (int64, error) {
	f, err := parseRequiredFloat(args, tool, field)
	if err != nil {
		return 0, err
	}
	if f < 1 {
		return 0, fmt.Errorf("%s: %s must be positive", tool, field)
	}
	return int64(f), nil
}

func parseRequiredFloat(args map[string]any, tool, field string) (float64, error) {
	if args == nil {
		return 0, fmt.Errorf("%s: arguments required", tool)
	}
	raw, ok := args[field]
	if !ok {
		return 0, fmt.Errorf("%s: %s is required", tool, field)
	}
	switch v := raw.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("%s: %s must be a number", tool, field)
	}
}

func parseOptionalFloat(args map[string]any, field string, fallback float64) (float64, error) {
	if args == nil {
		return fallback, nil
	}
	raw, ok := args[field]
	if !ok || raw == nil {
		return fallback, nil
	}
	switch v := raw.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("%s must be a number", field)
	}
}

func parseOptionalFloatMaybe(args map[string]any, field string) (float64, bool, error) {
	if args == nil {
		return 0, false, nil
	}
	raw, ok := args[field]
	if !ok || raw == nil {
		return 0, false, nil
	}
	switch v := raw.(type) {
	case float64:
		return v, true, nil
	case int:
		return float64(v), true, nil
	default:
		return 0, false, fmt.Errorf("%s must be a number", field)
	}
}

func parseOptionalInt(args map[string]any, field string, fallback int) (int, error) {
	if args == nil {
		return fallback, nil
	}
	raw, ok := args[field]
	if !ok || raw == nil {
		return fallback, nil
	}
	switch v := raw.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("%s must be an integer", field)
	}
}

func parseOptionalBool(args map[string]any, field string, fallback bool) (bool, error) {
	if args == nil {
		return fallback, nil
	}
	raw, ok := args[field]
	if !ok || raw == nil {
		return fallback, nil
	}
	v, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", field)
	}
	return v, nil
}

func parseOptionalTime(args map[string]any, field string) (time.Time, bool, error) {
	if args == nil {
		return time.Time{}, false, nil
	}
	raw, ok := args[field]
	if !ok || raw == nil {
		return time.Time{}, false, nil
	}
	s, ok := raw.(string)
	if !ok {
		return time.Time{}, false, fmt.Errorf("%s must be RFC3339 string", field)
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, false, fmt.Errorf("%s parse RFC3339: %w", field, err)
	}
	return t.UTC(), true, nil
}
