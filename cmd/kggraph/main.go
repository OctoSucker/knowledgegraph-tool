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
	case "add-edge":
		runAddEdge(ctx, args)
	case "add-edges-batch":
		runAddEdgesBatch(ctx, args)
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
	case "-h", "--help", "help":
		writeJSONAndExit(0, map[string]any{"usage": usage()})
	default:
		writeJSONAndExit(2, map[string]any{"error": fmt.Sprintf("unknown command %q", cmd), "usage": usage()})
	}
}

func usage() string {
	return "commands: add-edge, add-edges-batch, lookup-node-exact, lookup-node-semantic, list-nodes, list-edges, call, serve-mcp"
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

func runAddEdge(ctx context.Context, argv []string) {
	fs := flag.NewFlagSet("add-edge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cfg commonFlags
	addCommonFlags(fs, &cfg)
	var fromID, toID string
	var positive bool
	fs.StringVar(&fromID, "from-id", "", "source node id")
	fs.StringVar(&toID, "to-id", "", "target node id")
	fs.BoolVar(&positive, "positive", true, "positive correlation")
	mustParse(fs, argv)
	svc, err := openService(cfg)
	exitOnOpenError(err)
	defer svc.Close()
	out, err := svc.Call(ctx, kggraph.ToolAddEdge, map[string]any{
		"from_id":  fromID,
		"to_id":    toID,
		"positive": positive,
	})
	writeResult(err, out)
}

func runAddEdgesBatch(ctx context.Context, argv []string) {
	fs := flag.NewFlagSet("add-edges-batch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cfg commonFlags
	addCommonFlags(fs, &cfg)
	var edgesJSON string
	fs.StringVar(&edgesJSON, "edges-json", "", "JSON array of edges")
	mustParse(fs, argv)
	var edges []map[string]any
	if err := json.Unmarshal([]byte(edgesJSON), &edges); err != nil {
		writeJSONAndExit(2, map[string]any{"error": fmt.Sprintf("parse edges-json: %v", err)})
	}
	svc, err := openService(cfg)
	exitOnOpenError(err)
	defer svc.Close()
	out, err := svc.Call(ctx, kggraph.ToolAddEdgesBatch, map[string]any{"edges": anySlice(edges)})
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
