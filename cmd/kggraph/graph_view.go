package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	kggraph "github.com/OctoSucker/KGgraph"
)

func runGraphView(ctx context.Context, argv []string) {
	fs := flag.NewFlagSet("graph-view", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cfg commonFlags
	addCommonFlags(fs, &cfg)
	var host string
	var port int
	fs.StringVar(&host, "host", "127.0.0.1", "bind host")
	fs.IntVar(&port, "port", 8787, "bind port")
	mustParse(fs, argv)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(graphViewHTML))
	})
	mux.HandleFunc("/api/graph", func(w http.ResponseWriter, r *http.Request) {
		handleGraphAPI(ctx, cfg, w, r)
	})
	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Fprintf(os.Stderr, "kggraph graph-view listening on http://%s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		writeJSONAndExit(1, map[string]any{"error": err.Error()})
	}
}

func handleGraphAPI(ctx context.Context, cfg commonFlags, w http.ResponseWriter, r *http.Request) {
	svc, err := openService(cfg)
	if err != nil {
		writeAPIError(w, 500, err)
		return
	}
	defer svc.Close()

	nodesOut, err := svc.Call(ctx, kggraph.ToolListNodes, map[string]any{})
	if err != nil {
		writeAPIError(w, 500, err)
		return
	}
	edgesOut, err := svc.Call(ctx, kggraph.ToolListEdges, map[string]any{})
	if err != nil {
		writeAPIError(w, 500, err)
		return
	}

	nodesRaw, _ := nodesOut["nodes"].([]map[string]any)
	edgesRaw, _ := edgesOut["edges"].([]map[string]any)

	startID := strings.TrimSpace(r.URL.Query().Get("start_id"))
	maxDepth := parsePositiveIntOrDefault(r.URL.Query().Get("max_depth"), 2)
	graphKind := strings.TrimSpace(r.URL.Query().Get("graph_kind"))
	filteredNodes, filteredEdges := filterSubgraph(nodesRaw, edgesRaw, startID, maxDepth, graphKind)

	payload := map[string]any{
		"nodes":       filteredNodes,
		"edges":       filteredEdges,
		"total_nodes": len(nodesRaw),
		"total_edges": len(edgesRaw),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func filterSubgraph(nodes, edges []map[string]any, startID string, maxDepth int, graphKind string) ([]map[string]any, []map[string]any) {
	if maxDepth < 1 {
		maxDepth = 1
	}
	adj := map[string][]string{}
	filteredEdges := make([]map[string]any, 0, len(edges))
	for _, e := range edges {
		if graphKind != "" {
			if k, _ := e["graph_kind"].(string); strings.TrimSpace(k) != graphKind {
				continue
			}
		}
		fromID, _ := e["from_id"].(string)
		toID, _ := e["to_id"].(string)
		if fromID == "" || toID == "" {
			continue
		}
		filteredEdges = append(filteredEdges, e)
		adj[fromID] = append(adj[fromID], toID)
	}
	if startID == "" {
		return nodes, filteredEdges
	}
	visited := map[string]int{startID: 0}
	q := []string{startID}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		d := visited[cur]
		if d >= maxDepth {
			continue
		}
		for _, next := range adj[cur] {
			if _, ok := visited[next]; ok {
				continue
			}
			visited[next] = d + 1
			q = append(q, next)
		}
	}
	outNodes := make([]map[string]any, 0, len(visited))
	for _, n := range nodes {
		id, _ := n["id"].(string)
		if _, ok := visited[id]; ok {
			outNodes = append(outNodes, n)
		}
	}
	outEdges := make([]map[string]any, 0, len(filteredEdges))
	for _, e := range filteredEdges {
		fromID, _ := e["from_id"].(string)
		toID, _ := e["to_id"].(string)
		_, okFrom := visited[fromID]
		_, okTo := visited[toID]
		if okFrom && okTo {
			outEdges = append(outEdges, e)
		}
	}
	return outNodes, outEdges
}

func parsePositiveIntOrDefault(v string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func writeAPIError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
}

const graphViewHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <title>KGgraph Viewer</title>
  <script src="https://unpkg.com/vis-network/standalone/umd/vis-network.min.js"></script>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 0; }
    .bar { padding: 10px; border-bottom: 1px solid #e5e5e5; display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
    #graph { width: 100vw; height: calc(100vh - 58px); }
    input, select, button { padding: 6px 8px; }
    .meta { color: #666; font-size: 12px; margin-left: auto; }
  </style>
</head>
<body>
  <div class="bar">
    <label>start-id <input id="startId" placeholder="optional node id"/></label>
    <label>max-depth <input id="maxDepth" type="number" min="1" value="2" style="width:70px"/></label>
    <label>graph-kind <input id="graphKind" placeholder="knowledge / skill"/></label>
    <button id="refreshBtn">Refresh</button>
    <span class="meta" id="meta"></span>
  </div>
  <div id="graph"></div>
  <script>
    const container = document.getElementById("graph");
    const network = new vis.Network(container, {nodes: [], edges: []}, {
      physics: { stabilization: false },
      edges: { arrows: { to: { enabled: true, scaleFactor: 0.5 } }, smooth: { type: "dynamic" } }
    });
    async function refresh() {
      const startId = document.getElementById("startId").value.trim();
      const maxDepth = document.getElementById("maxDepth").value.trim();
      const graphKind = document.getElementById("graphKind").value.trim();
      const q = new URLSearchParams();
      if (startId) q.set("start_id", startId);
      if (maxDepth) q.set("max_depth", maxDepth);
      if (graphKind) q.set("graph_kind", graphKind);
      const res = await fetch("/api/graph?" + q.toString());
      const data = await res.json();
      if (!res.ok) {
        alert(data.error || "failed to load graph");
        return;
      }
      const nodes = data.nodes.map(n => ({
        id: n.id,
        label: n.id,
        title: "type: " + (n.node_type || "") + "\nstatus: " + (n.status || "")
      }));
      const edges = data.edges.map(e => ({
        id: e.id,
        from: e.from_id,
        to: e.to_id,
        label: e.relation_type || "",
        title:
          "kind: " + (e.graph_kind || "") + "\n" +
          "confidence: " + (e.confidence ?? "") + "\n" +
          "condition: " + (e.condition_text || "") + "\n" +
          "evidence: " + (e.evidence_count ?? 0) + ", failed: " + (e.failed_count ?? 0)
      }));
      network.setData({nodes, edges});
      document.getElementById("meta").innerText = "showing " + nodes.length + "/" + data.total_nodes + " nodes, " + edges.length + "/" + data.total_edges + " edges";
    }
    document.getElementById("refreshBtn").addEventListener("click", refresh);
    refresh();
  </script>
</body>
</html>`
