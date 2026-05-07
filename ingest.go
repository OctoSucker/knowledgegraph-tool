package knowledgegraph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openai "github.com/openai/openai-go"
)

func (s *Service) IngestStatement(ctx context.Context, statement, graphKind, sourceType, sourceRef, model string, defaultConfidence float64) (map[string]any, error) {
	statement = strings.TrimSpace(statement)
	if statement == "" {
		return nil, fmt.Errorf("knowledgegraph: ingest statement: empty statement")
	}
	if strings.TrimSpace(graphKind) == "" {
		graphKind = "knowledge"
	}
	if strings.TrimSpace(sourceType) == "" {
		sourceType = "llm_extracted"
	}
	if strings.TrimSpace(model) == "" {
		model = IngestModelFromEnv()
	}
	if defaultConfidence <= 0 || defaultConfidence > 1 {
		defaultConfidence = DefaultIngestConfidence
	}
	nodes, edges, err := s.extractWithLLM(ctx, statement, model, defaultConfidence)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 && len(edges) == 0 {
		return nil, fmt.Errorf("knowledgegraph: ingest statement: no nodes/edges extracted")
	}
	nodeSet := map[string]struct{}{}
	for _, n := range nodes {
		id := strings.TrimSpace(n.ID)
		if id == "" {
			continue
		}
		nodeType := strings.TrimSpace(n.NodeType)
		if nodeType == "" {
			nodeType = "entity"
		}
		if err := s.graph.UpsertNode(ctx, NodeUpsert{
			ID:       id,
			NodeType: nodeType,
			Aliases:  n.Aliases,
			Status:   "active",
		}); err != nil {
			return nil, err
		}
		nodeSet[id] = struct{}{}
	}
	addedEdges := make([]map[string]any, 0, len(edges))
	for _, e := range edges {
		fromID := strings.TrimSpace(e.FromID)
		toID := strings.TrimSpace(e.ToID)
		if fromID == "" || toID == "" {
			continue
		}
		if _, ok := nodeSet[fromID]; !ok {
			if err := s.graph.UpsertNode(ctx, NodeUpsert{ID: fromID, NodeType: "entity", Status: "active"}); err != nil {
				return nil, err
			}
			nodeSet[fromID] = struct{}{}
		}
		if _, ok := nodeSet[toID]; !ok {
			if err := s.graph.UpsertNode(ctx, NodeUpsert{ID: toID, NodeType: "entity", Status: "active"}); err != nil {
				return nil, err
			}
			nodeSet[toID] = struct{}{}
		}
		relation := strings.TrimSpace(e.RelationType)
		if relation == "" {
			relation = "related_to"
		}
		conf := e.Confidence
		if conf <= 0 || conf > 1 {
			conf = defaultConfidence
		}
		polarity := e.Polarity
		if polarity < -1 || polarity > 1 {
			polarity = 1
		}
		edgeID, err := s.graph.UpsertEdge(ctx, EdgeUpsert{
			FromID:        fromID,
			ToID:          toID,
			GraphKind:     graphKind,
			RelationType:  relation,
			Polarity:      polarity,
			Confidence:    conf,
			ConditionText: strings.TrimSpace(e.ConditionText),
			SourceType:    sourceType,
			SourceRef:     sourceRef,
			ObservedAt:    e.ObservedAt,
			ValidFrom:     e.ValidFrom,
			ValidUntil:    e.ValidUntil,
		})
		if err != nil {
			return nil, err
		}
		addedEdges = append(addedEdges, map[string]any{
			"edge_id":      edgeID,
			"from_id":      fromID,
			"to_id":        toID,
			"relation_type": relation,
			"confidence":   conf,
		})
	}
	nodeIDs := make([]string, 0, len(nodeSet))
	for id := range nodeSet {
		nodeIDs = append(nodeIDs, id)
	}
	return map[string]any{
		"statement":     statement,
		"graph_kind":    graphKind,
		"source_type":   sourceType,
		"node_count":    len(nodeIDs),
		"edge_count":    len(addedEdges),
		"node_ids":      nodeIDs,
		"edges":         addedEdges,
		"ingest_model":  model,
	}, nil
}

func (s *Service) extractWithLLM(ctx context.Context, statement, model string, defaultConfidence float64) ([]NodeUpsert, []EdgeUpsert, error) {
	if s == nil || s.graph == nil || s.graph.embedder == nil {
		return nil, nil, fmt.Errorf("knowledgegraph: ingest statement: OpenAI embedder is required")
	}
	emb, ok := s.graph.embedder.(*OpenAIEmbedder)
	if !ok || emb == nil {
		return nil, nil, fmt.Errorf("knowledgegraph: ingest statement: only OpenAI embedder supports llm extraction")
	}
	systemPrompt := "Extract a compact knowledge graph from the user statement. Return STRICT JSON only. " +
		`Schema: {"nodes":[{"id":"string","node_type":"entity|event|concept","aliases":["string"]}],"edges":[{"from":"string","to":"string","relation":"related_to|causes|increases_probability_of|decreases_probability_of|requires|blocks|supports|contradicts|part_of|example_of","polarity":-1|0|1,"confidence":0..1,"condition":"string","observed_at":"RFC3339 or empty","valid_from":"RFC3339 or empty","valid_until":"RFC3339 or empty"}]}. ` +
		"Extract only relationships explicitly stated or strongly implied by the statement. Do not add outside knowledge. " +
		"Use short canonical node IDs in the original language; prefer noun phrases, events, or concepts, not full sentences. Merge duplicate concepts within the statement. " +
		"Use relation only from the allowed enum. Use polarity 1 for supporting/positive relations, -1 for opposing/negative relations, 0 for neutral structural relations. " +
		"Confidence guide: 0.75-0.90 for explicit relations, 0.55-0.70 for strong implications, 0.35-0.55 for weak/speculative claims. " +
		"Only fill observed_at, valid_from, or valid_until when the statement explicitly contains a date/time or validity window. Never invent dates. Use empty strings otherwise."
	userPrompt := fmt.Sprintf("Default confidence if uncertain: %.2f\nStatement:\n%s", defaultConfidence, statement)
	resp, err := emb.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel(model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.ChatCompletionMessageParamUnion{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: openai.String(systemPrompt),
					},
				},
			},
			openai.ChatCompletionMessageParamUnion{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(userPrompt),
					},
				},
			},
		},
		Temperature: openai.Float(0.1),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("knowledgegraph: ingest statement llm call: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, nil, fmt.Errorf("knowledgegraph: ingest statement: empty model response")
	}
	raw := strings.TrimSpace(resp.Choices[0].Message.Content)
	if raw == "" {
		return nil, nil, fmt.Errorf("knowledgegraph: ingest statement: empty model content")
	}
	var payload struct {
		Nodes []struct {
			ID       string   `json:"id"`
			NodeType string   `json:"node_type"`
			Aliases  []string `json:"aliases"`
		} `json:"nodes"`
		Edges []struct {
			From       string  `json:"from"`
			To         string  `json:"to"`
			Relation   string  `json:"relation"`
			Polarity   int     `json:"polarity"`
			Confidence float64 `json:"confidence"`
			Condition  string  `json:"condition"`
			ObservedAt string  `json:"observed_at"`
			ValidFrom  string  `json:"valid_from"`
			ValidUntil string  `json:"valid_until"`
		} `json:"edges"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, nil, fmt.Errorf("knowledgegraph: ingest statement json parse: %w", err)
	}
	nodes := make([]NodeUpsert, 0, len(payload.Nodes))
	for _, n := range payload.Nodes {
		nodes = append(nodes, NodeUpsert{
			ID:       n.ID,
			NodeType: n.NodeType,
			Aliases:  n.Aliases,
			Status:   "active",
		})
	}
	edges := make([]EdgeUpsert, 0, len(payload.Edges))
	for _, e := range payload.Edges {
		observedAt, err := parseLLMTime(e.ObservedAt)
		if err != nil {
			return nil, nil, fmt.Errorf("knowledgegraph: ingest statement: invalid observed_at %q: %w", e.ObservedAt, err)
		}
		validFrom, err := parseLLMTime(e.ValidFrom)
		if err != nil {
			return nil, nil, fmt.Errorf("knowledgegraph: ingest statement: invalid valid_from %q: %w", e.ValidFrom, err)
		}
		validUntil, err := parseLLMTime(e.ValidUntil)
		if err != nil {
			return nil, nil, fmt.Errorf("knowledgegraph: ingest statement: invalid valid_until %q: %w", e.ValidUntil, err)
		}
		edges = append(edges, EdgeUpsert{
			FromID:        e.From,
			ToID:          e.To,
			RelationType:  e.Relation,
			Polarity:      e.Polarity,
			Confidence:    e.Confidence,
			ConditionText: e.Condition,
			ObservedAt:    observedAt,
			ValidFrom:     validFrom,
			ValidUntil:    validUntil,
		})
	}
	return nodes, edges, nil
}

func parseLLMTime(raw string) (*time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	tt := t.UTC()
	return &tt, nil
}
