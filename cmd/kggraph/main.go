package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	kggraph "github.com/OctoSucker/KGgraph"
)

type commonFlags struct {
	workspace      string
	dbPath         string
	baseURL        string
	apiKey         string
	embeddingModel string
}

func main() {
	if len(os.Args) < 2 {
		writeJSONAndExit(2, map[string]any{"error": usage()})
	}
	ctx := context.Background()
	cmd := strings.TrimSpace(os.Args[1])
	args := os.Args[2:]

	switch cmd {
	case "upsert-node":
		runUpsertNode(ctx, args)
	case "add-fact-edge":
		runAddEdge(ctx, args, kggraph.ToolAddFactEdge)
	case "add-skill-edge":
		runAddEdge(ctx, args, kggraph.ToolAddSkillEdge)
	case "ingest-statement":
		runIngestStatement(ctx, args)
	case "attach-edge-evidence":
		runAttachEdgeEvidence(ctx, args)
	case "verify-edge":
		runVerifyEdge(ctx, args)
	case "expand-reasoning":
		runExpandReasoning(ctx, args)
	case "lookup-node-exact":
		runLookup(ctx, args, kggraph.ToolLookupNodeExact)
	case "lookup-node-semantic":
		runLookup(ctx, args, kggraph.ToolLookupNodeSemantic)
	case "list-nodes":
		runList(ctx, args, kggraph.ToolListNodes)
	case "list-edges":
		runList(ctx, args, kggraph.ToolListEdges)
	case "call":
		runCall(ctx, args)
	case "serve-mcp":
		runServeMCP(ctx, args)
	case "graph-view":
		runGraphView(ctx, args)
	case "-h", "--help", "help":
		writeJSONAndExit(0, map[string]any{"usage": usage()})
	default:
		writeJSONAndExit(2, map[string]any{"error": fmt.Sprintf("unknown command %q", cmd), "usage": usage()})
	}
}

func usage() string {
	return "commands: upsert-node, add-fact-edge, add-skill-edge, ingest-statement, attach-edge-evidence, verify-edge, expand-reasoning, lookup-node-exact, lookup-node-semantic, list-nodes, list-edges, call, serve-mcp, graph-view"
}

func addCommonFlags(fs *flag.FlagSet, c *commonFlags) {
	fs.StringVar(&c.workspace, "workspace", "", "workspace root containing data/knowledgegraph.sqlite")
	fs.StringVar(&c.dbPath, "db", os.Getenv("KG_DB_PATH"), "direct sqlite file path")
	fs.StringVar(&c.baseURL, "base-url", getenvDefault("OPENAI_BASE_URL", ""), "OpenAI base URL")
	fs.StringVar(&c.apiKey, "api-key", getenvDefault("OPENAI_API_KEY", ""), "OpenAI API key")
	fs.StringVar(&c.embeddingModel, "embedding-model", getenvDefault("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"), "OpenAI embedding model")
}

func openService(cfg commonFlags) (*kggraph.Service, error) {
	store, err := kggraph.OpenStore(kggraph.StoreConfig{
		WorkspaceRoot: cfg.workspace,
		DBPath:        cfg.dbPath,
	})
	if err != nil {
		return nil, err
	}
	var embedder kggraph.Embedder
	if strings.TrimSpace(cfg.embeddingModel) != "" && (strings.TrimSpace(cfg.apiKey) != "" || strings.TrimSpace(cfg.baseURL) != "") {
		embedder = kggraph.NewOpenAIEmbedder(kggraph.OpenAIConfig{
			BaseURL:        cfg.baseURL,
			APIKey:         cfg.apiKey,
			EmbeddingModel: cfg.embeddingModel,
		})
	}
	return kggraph.NewService(store, embedder)
}

func runUpsertNode(ctx context.Context, argv []string) {
	fs := flag.NewFlagSet("upsert-node", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cfg commonFlags
	addCommonFlags(fs, &cfg)
	var id, nodeType, status string
	var aliasesJSON string
	fs.StringVar(&id, "id", "", "node id")
	fs.StringVar(&nodeType, "node-type", "entity", "node type")
	fs.StringVar(&status, "status", "active", "node status")
	fs.StringVar(&aliasesJSON, "aliases-json", "[]", "JSON array of aliases")
	mustParse(fs, argv)
	var aliases []string
	if err := json.Unmarshal([]byte(aliasesJSON), &aliases); err != nil {
		writeJSONAndExit(2, map[string]any{"error": fmt.Sprintf("parse aliases-json: %v", err)})
	}
	svc, err := openService(cfg)
	exitOnOpenError(err)
	defer svc.Close()
	out, err := svc.Call(ctx, kggraph.ToolUpsertNode, map[string]any{
		"id":        id,
		"node_type": nodeType,
		"status":    status,
		"aliases":   anySlice(aliases),
	})
	writeResult(err, out)
}

func runAddEdge(ctx context.Context, argv []string, tool string) {
	fs := flag.NewFlagSet(tool, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cfg commonFlags
	addCommonFlags(fs, &cfg)
	var fromID, toID, relationType, conditionText, sourceType, sourceRef, observedAt, validFrom, validUntil, expiresAt, activationRule string
	var polarity, decayHalfLifeDays int
	var confidence float64
	fs.StringVar(&fromID, "from-id", "", "source node id")
	fs.StringVar(&toID, "to-id", "", "target node id")
	fs.StringVar(&relationType, "relation-type", "", "relation type")
	fs.IntVar(&polarity, "polarity", 1, "relation polarity: -1, 0, 1")
	fs.Float64Var(&confidence, "confidence", 0.6, "edge confidence in [0,1]")
	fs.StringVar(&conditionText, "condition-text", "", "condition text")
	fs.StringVar(&sourceType, "source-type", "", "source type")
	fs.StringVar(&sourceRef, "source-ref", "", "source reference")
	fs.StringVar(&observedAt, "observed-at", "", "optional RFC3339 observed timestamp")
	fs.StringVar(&validFrom, "valid-from", "", "optional RFC3339 validity start")
	fs.StringVar(&validUntil, "valid-until", "", "optional RFC3339 validity end")
	fs.IntVar(&decayHalfLifeDays, "decay-half-life-days", 30, "time decay half-life in days")
	fs.StringVar(&expiresAt, "expires-at", "", "optional RFC3339 expiration")
	fs.StringVar(&activationRule, "activation-rule", "", "skill activation rule (required for add-skill-edge)")
	mustParse(fs, argv)
	args := map[string]any{
		"from_id":              fromID,
		"to_id":                toID,
		"relation_type":        relationType,
		"polarity":             polarity,
		"confidence":           confidence,
		"condition_text":       conditionText,
		"source_type":          sourceType,
		"source_ref":           sourceRef,
		"decay_half_life_days": decayHalfLifeDays,
	}
	if strings.TrimSpace(expiresAt) != "" {
		args["expires_at"] = expiresAt
	}
	if strings.TrimSpace(observedAt) != "" {
		args["observed_at"] = observedAt
	}
	if strings.TrimSpace(validFrom) != "" {
		args["valid_from"] = validFrom
	}
	if strings.TrimSpace(validUntil) != "" {
		args["valid_until"] = validUntil
	}
	if tool == kggraph.ToolAddSkillEdge {
		args["activation_rule"] = activationRule
	}
	svc, err := openService(cfg)
	exitOnOpenError(err)
	defer svc.Close()
	out, err := svc.Call(ctx, tool, args)
	writeResult(err, out)
}

func runIngestStatement(ctx context.Context, argv []string) {
	fs := flag.NewFlagSet("ingest-statement", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cfg commonFlags
	addCommonFlags(fs, &cfg)
	var statement, graphKind, sourceType, sourceRef, model string
	var defaultConfidence float64
	fs.StringVar(&statement, "statement", "", "natural-language statement to ingest")
	fs.StringVar(&graphKind, "graph-kind", "knowledge", "graph kind filter")
	fs.StringVar(&sourceType, "source-type", "llm_extracted", "source type")
	fs.StringVar(&sourceRef, "source-ref", "", "source ref")
	fs.StringVar(&model, "model", getenvDefault("OPENAI_MODEL", kggraph.DefaultIngestModel), "LLM model used for extraction")
	fs.Float64Var(&defaultConfidence, "default-confidence", kggraph.DefaultIngestConfidence, "fallback confidence when LLM is uncertain")
	mustParse(fs, argv)
	args := map[string]any{
		"statement":          statement,
		"graph_kind":         graphKind,
		"source_type":        sourceType,
		"source_ref":         sourceRef,
		"model":              model,
		"default_confidence": defaultConfidence,
	}
	svc, err := openService(cfg)
	exitOnOpenError(err)
	defer svc.Close()
	out, err := svc.Call(ctx, kggraph.ToolIngestStatement, args)
	writeResult(err, out)
}

func runAttachEdgeEvidence(ctx context.Context, argv []string) {
	fs := flag.NewFlagSet("attach-edge-evidence", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cfg commonFlags
	addCommonFlags(fs, &cfg)
	var edgeID int64
	var sourceType, sourceRef, snippet, observedAt string
	var supports bool
	var weight float64
	fs.Int64Var(&edgeID, "edge-id", 0, "edge id")
	fs.StringVar(&sourceType, "source-type", "", "source type")
	fs.StringVar(&sourceRef, "source-ref", "", "source ref")
	fs.StringVar(&snippet, "snippet", "", "evidence snippet")
	fs.BoolVar(&supports, "supports", true, "whether evidence supports the edge")
	fs.Float64Var(&weight, "weight", 1.0, "evidence weight")
	fs.StringVar(&observedAt, "observed-at", "", "optional RFC3339 observed timestamp")
	mustParse(fs, argv)
	args := map[string]any{
		"edge_id":     edgeID,
		"source_type": sourceType,
		"source_ref":  sourceRef,
		"snippet":     snippet,
		"supports":    supports,
		"weight":      weight,
	}
	if strings.TrimSpace(observedAt) != "" {
		args["observed_at"] = observedAt
	}
	svc, err := openService(cfg)
	exitOnOpenError(err)
	defer svc.Close()
	out, err := svc.Call(ctx, kggraph.ToolAttachEdgeEvidence, args)
	writeResult(err, out)
}

func runVerifyEdge(ctx context.Context, argv []string) {
	fs := flag.NewFlagSet("verify-edge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cfg commonFlags
	addCommonFlags(fs, &cfg)
	var edgeID int64
	var success bool
	var confidence float64
	var setConfidence bool
	var verifiedAt string
	fs.Int64Var(&edgeID, "edge-id", 0, "edge id")
	fs.BoolVar(&success, "success", true, "verification result")
	fs.Float64Var(&confidence, "confidence", 0, "optional confidence value")
	fs.BoolVar(&setConfidence, "set-confidence", false, "whether to update confidence")
	fs.StringVar(&verifiedAt, "verified-at", "", "optional RFC3339 verified timestamp")
	mustParse(fs, argv)
	args := map[string]any{"edge_id": edgeID, "success": success}
	if setConfidence {
		args["confidence"] = confidence
	}
	if strings.TrimSpace(verifiedAt) != "" {
		args["verified_at"] = verifiedAt
	}
	svc, err := openService(cfg)
	exitOnOpenError(err)
	defer svc.Close()
	out, err := svc.Call(ctx, kggraph.ToolVerifyEdge, args)
	writeResult(err, out)
}

func runExpandReasoning(ctx context.Context, argv []string) {
	fs := flag.NewFlagSet("expand-reasoning", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cfg commonFlags
	addCommonFlags(fs, &cfg)
	var startID, graphKind, asOf string
	var maxDepth, maxBranch, maxResults int
	var includeNegative bool
	var minScore float64
	fs.StringVar(&startID, "start-id", "", "start node id")
	fs.StringVar(&graphKind, "graph-kind", "knowledge", "graph kind filter")
	fs.StringVar(&asOf, "as-of", "", "optional RFC3339 query time")
	fs.IntVar(&maxDepth, "max-depth", 3, "max reasoning depth")
	fs.IntVar(&maxBranch, "max-branch", 5, "max outgoing edges per step")
	fs.IntVar(&maxResults, "max-results", 10, "max result nodes")
	fs.BoolVar(&includeNegative, "include-negative", false, "whether to include negative-polarity edges")
	fs.Float64Var(&minScore, "min-score", 0, "minimum propagated score to keep")
	mustParse(fs, argv)
	args := map[string]any{
		"start_id":         startID,
		"graph_kind":       graphKind,
		"max_depth":        maxDepth,
		"max_branch":       maxBranch,
		"max_results":      maxResults,
		"include_negative": includeNegative,
		"min_score":        minScore,
	}
	if strings.TrimSpace(asOf) != "" {
		args["as_of"] = asOf
	}
	svc, err := openService(cfg)
	exitOnOpenError(err)
	defer svc.Close()
	out, err := svc.Call(ctx, kggraph.ToolExpandReasoning, args)
	writeResult(err, out)
}

func runLookup(ctx context.Context, argv []string, tool string) {
	fs := flag.NewFlagSet(tool, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cfg commonFlags
	addCommonFlags(fs, &cfg)
	var term string
	fs.StringVar(&term, "term", "", "term to resolve")
	mustParse(fs, argv)
	svc, err := openService(cfg)
	exitOnOpenError(err)
	defer svc.Close()
	out, err := svc.Call(ctx, tool, map[string]any{"term": term})
	writeResult(err, out)
}

func runList(ctx context.Context, argv []string, tool string) {
	fs := flag.NewFlagSet(tool, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cfg commonFlags
	addCommonFlags(fs, &cfg)
	mustParse(fs, argv)
	svc, err := openService(cfg)
	exitOnOpenError(err)
	defer svc.Close()
	out, err := svc.Call(ctx, tool, map[string]any{})
	writeResult(err, out)
}

func runCall(ctx context.Context, argv []string) {
	fs := flag.NewFlagSet("call", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cfg commonFlags
	addCommonFlags(fs, &cfg)
	var tool string
	var argsJSON string
	fs.StringVar(&tool, "tool", "", "tool name")
	fs.StringVar(&argsJSON, "args-json", "{}", "tool arguments as JSON object")
	mustParse(fs, argv)

	var argsMap map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &argsMap); err != nil {
		writeJSONAndExit(2, map[string]any{"error": fmt.Sprintf("parse args-json: %v", err)})
	}
	svc, err := openService(cfg)
	exitOnOpenError(err)
	defer svc.Close()
	out, err := svc.Call(ctx, tool, argsMap)
	writeResult(err, out)
}

func runServeMCP(ctx context.Context, argv []string) {
	fs := flag.NewFlagSet("serve-mcp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cfg commonFlags
	addCommonFlags(fs, &cfg)
	mustParse(fs, argv)
	svc, err := openService(cfg)
	exitOnOpenError(err)
	defer svc.Close()
	if err := kggraph.RunMCPServer(ctx, svc); err != nil {
		writeJSONAndExit(1, map[string]any{"error": err.Error()})
	}
}

func mustParse(fs *flag.FlagSet, argv []string) {
	if err := fs.Parse(argv); err != nil {
		writeJSONAndExit(2, map[string]any{"error": err.Error()})
	}
}

func exitOnOpenError(err error) {
	if err != nil {
		writeJSONAndExit(1, map[string]any{"error": err.Error()})
	}
}

func writeResult(err error, out map[string]any) {
	if err != nil {
		writeJSONAndExit(1, map[string]any{"error": err.Error()})
	}
	writeJSONAndExit(0, out)
}

func writeJSONAndExit(code int, payload map[string]any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
	os.Exit(code)
}

func getenvDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func anySlice[T any](in []T) []any {
	out := make([]any, len(in))
	for i := range in {
		out[i] = in[i]
	}
	return out
}
