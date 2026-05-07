package knowledgegraph

import (
	"context"
	"fmt"
	"strings"
)

const (
	ToolAddEdge            = "kg_add_edge"
	ToolAddEdgesBatch      = "kg_add_edges_batch"
	ToolLookupNodeExact    = "kg_lookup_node_exact"
	ToolLookupNodeSemantic = "kg_lookup_node_semantic"
	ToolListNodes          = "kg_list_nodes"
	ToolListEdges          = "kg_list_edges"
)

type Service struct {
	store *Store
	graph *Graph
}

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
		ToolAddEdge,
		ToolAddEdgesBatch,
		ToolLookupNodeExact,
		ToolLookupNodeSemantic,
		ToolListNodes,
		ToolListEdges,
	}
}

func (s *Service) Call(ctx context.Context, tool string, arguments map[string]any) (map[string]any, error) {
	switch strings.TrimSpace(tool) {
	case ToolAddEdge:
		fromID, err := parseRequiredString(arguments, tool, "from_id")
		if err != nil {
			return nil, err
		}
		toID, err := parseRequiredString(arguments, tool, "to_id")
		if err != nil {
			return nil, err
		}
		positive, err := parseOptionalPositive(arguments, tool)
		if err != nil {
			return nil, err
		}
		if err := s.graph.EnsureNode(ctx, fromID); err != nil {
			return nil, err
		}
		if err := s.graph.EnsureNode(ctx, toID); err != nil {
			return nil, err
		}
		if err := s.graph.AddEdge(fromID, toID, positive); err != nil {
			return nil, err
		}
		return map[string]any{"from_id": fromID, "to_id": toID, "positive": positive}, nil
	case ToolAddEdgesBatch:
		edges, err := parseBatchEdges(arguments)
		if err != nil {
			return nil, err
		}
		added := make([]map[string]any, 0, len(edges))
		for i, e := range edges {
			if err := s.graph.EnsureNode(ctx, e.FromID); err != nil {
				return nil, fmt.Errorf("%s: edge[%d]: ensure from_id %q: %w", tool, i, e.FromID, err)
			}
			if err := s.graph.EnsureNode(ctx, e.ToID); err != nil {
				return nil, fmt.Errorf("%s: edge[%d]: ensure to_id %q: %w", tool, i, e.ToID, err)
			}
			if err := s.graph.AddEdge(e.FromID, e.ToID, e.Positive); err != nil {
				return nil, fmt.Errorf("%s: edge[%d]: add edge %q -> %q: %w", tool, i, e.FromID, e.ToID, err)
			}
			added = append(added, map[string]any{
				"from_id":  e.FromID,
				"to_id":    e.ToID,
				"positive": e.Positive,
			})
		}
		return map[string]any{"added_count": len(added), "edges": added}, nil
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
		nodeIDs := make([]string, len(rows))
		for i, r := range rows {
			nodeIDs[i] = r.ID
		}
		return map[string]any{"node_ids": nodeIDs, "nodes": rows}, nil
	case ToolListEdges:
		rows, err := s.graph.AllEdges()
		if err != nil {
			return nil, err
		}
		edges := make([]map[string]any, len(rows))
		for i, e := range rows {
			edges[i] = map[string]any{"from_id": e.FromID, "to_id": e.ToID, "positive": e.Positive}
		}
		return map[string]any{"edges": edges}, nil
	default:
		return nil, fmt.Errorf("knowledgegraph: unknown tool %q", tool)
	}
}

func ToolSchema(tool string) map[string]any {
	switch tool {
	case ToolAddEdge:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"from_id":  map[string]any{"type": "string", "description": "Source node id; created with embedding if missing"},
				"to_id":    map[string]any{"type": "string", "description": "Target node id; created with embedding if missing"},
				"positive": map[string]any{"type": "boolean", "description": "True for positive correlation, false for negative; omit for true"},
			},
			"required":             []string{"from_id", "to_id"},
			"additionalProperties": false,
		}
	case ToolAddEdgesBatch:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"edges": map[string]any{
					"type":     "array",
					"items":    ToolSchema(ToolAddEdge),
					"minItems": 1,
				},
			},
			"required":             []string{"edges"},
			"additionalProperties": false,
		}
	case ToolLookupNodeExact, ToolLookupNodeSemantic:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"term": map[string]any{"type": "string", "description": "Phrase or canonical id to resolve"},
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
	default:
		return map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		}
	}
}

func ToolDescription(tool string) string {
	switch tool {
	case ToolAddEdge:
		return "Add a directed influence edge from_id -> to_id. Creates both endpoint nodes if they do not exist."
	case ToolAddEdgesBatch:
		return "Add multiple directed influence edges. Each edge creates endpoint nodes if missing."
	case ToolLookupNodeExact:
		return "Find a stored node id by exact string match on the term."
	case ToolLookupNodeSemantic:
		return "Find a node id: exact match first, else cosine similarity against stored node embeddings."
	case ToolListNodes:
		return "List all nodes: node_ids and nodes (each id plus embedding: base64 of little-endian float32 bytes when stored; empty nodes have no embedding field). For server-side similarity use kg_lookup_node_semantic."
	case ToolListEdges:
		return "List all directed edges (from_id, to_id, positive correlation flag)."
	default:
		return ""
	}
}

type batchEdge struct {
	FromID   string
	ToID     string
	Positive bool
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

func parseOptionalPositive(args map[string]any, tool string) (bool, error) {
	if args == nil {
		return true, nil
	}
	raw, ok := args["positive"]
	if !ok || raw == nil {
		return true, nil
	}
	switch v := raw.(type) {
	case bool:
		return v, nil
	case float64:
		if v == 0 {
			return false, nil
		}
		if v == 1 {
			return true, nil
		}
	case int:
		if v == 0 {
			return false, nil
		}
		if v == 1 {
			return true, nil
		}
	}
	return false, fmt.Errorf("%s: positive must be boolean", tool)
}

func parseBatchEdges(args map[string]any) ([]batchEdge, error) {
	rawEdges, ok := args["edges"]
	if !ok {
		return nil, fmt.Errorf("%s: edges is required", ToolAddEdgesBatch)
	}
	items, ok := rawEdges.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: edges must be an array", ToolAddEdgesBatch)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%s: edges must be non-empty", ToolAddEdgesBatch)
	}
	edges := make([]batchEdge, 0, len(items))
	for i, raw := range items {
		obj, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: edge[%d] must be an object", ToolAddEdgesBatch, i)
		}
		fromID, err := parseRequiredString(obj, ToolAddEdgesBatch, "from_id")
		if err != nil {
			return nil, fmt.Errorf("%s: edge[%d]: %w", ToolAddEdgesBatch, i, err)
		}
		toID, err := parseRequiredString(obj, ToolAddEdgesBatch, "to_id")
		if err != nil {
			return nil, fmt.Errorf("%s: edge[%d]: %w", ToolAddEdgesBatch, i, err)
		}
		positive, err := parseOptionalPositive(obj, ToolAddEdgesBatch)
		if err != nil {
			return nil, fmt.Errorf("%s: edge[%d]: %w", ToolAddEdgesBatch, i, err)
		}
		edges = append(edges, batchEdge{FromID: fromID, ToID: toID, Positive: positive})
	}
	return edges, nil
}
