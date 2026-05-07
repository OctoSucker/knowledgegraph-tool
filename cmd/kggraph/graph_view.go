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
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>KGgraph Viewer</title>
  <script src="https://unpkg.com/vis-network/standalone/umd/vis-network.min.js"></script>
  <style>
    :root {
      --bg: #eef1f7;
      --panel: #ffffff;
      --text: #1f2a37;
      --muted: #6b7280;
      --line: #e5e7eb;
      --primary: #2f80ed;
      --primary-hover: #1e67c7;
    }
    * { box-sizing: border-box; }
    html, body { width: 100%; height: 100%; margin: 0; }
    body {
      font-family: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      color: var(--text);
      background: var(--bg);
    }
    .app {
      height: 100%;
      display: flex;
      flex-direction: column;
      gap: 12px;
      padding: 12px;
    }
    .toolbar {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 12px;
      padding: 10px;
      display: flex;
      align-items: flex-end;
      flex-wrap: wrap;
      gap: 10px;
      box-shadow: 0 5px 20px rgba(31, 42, 55, 0.06);
    }
    .field {
      display: flex;
      flex-direction: column;
      gap: 4px;
      min-width: 130px;
    }
    .field label {
      color: var(--muted);
      font-size: 12px;
      line-height: 1;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.02em;
    }
    input {
      border: 1px solid var(--line);
      border-radius: 8px;
      background: #fff;
      color: var(--text);
      font-size: 14px;
      padding: 8px 10px;
      outline: none;
      transition: border-color .12s ease, box-shadow .12s ease;
    }
    input:focus {
      border-color: #93c5fd;
      box-shadow: 0 0 0 3px rgba(47, 128, 237, 0.16);
    }
    .actions {
      display: flex;
      gap: 8px;
      margin-left: auto;
      flex-wrap: wrap;
    }
    button {
      border: 1px solid var(--line);
      background: #fff;
      color: var(--text);
      border-radius: 8px;
      padding: 8px 12px;
      font-size: 13px;
      font-weight: 600;
      cursor: pointer;
    }
    button:hover { background: #f9fafb; }
    .btn-primary {
      border-color: var(--primary);
      background: var(--primary);
      color: #fff;
    }
    .btn-primary:hover { background: var(--primary-hover); }
    .btn-ghost {
      background: #f8fafc;
    }
    .stats {
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
    }
    .stat {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 12px;
      padding: 10px 12px;
      min-width: 150px;
      box-shadow: 0 5px 20px rgba(31, 42, 55, 0.06);
    }
    .stat .k {
      font-size: 11px;
      color: var(--muted);
      text-transform: uppercase;
      letter-spacing: 0.04em;
      font-weight: 700;
    }
    .stat .v {
      margin-top: 4px;
      font-size: 16px;
      font-weight: 700;
      color: var(--text);
    }
    .content {
      flex: 1;
      min-height: 360px;
      display: flex;
      gap: 12px;
    }
    .graph-wrap {
      position: relative;
      flex: 1;
      border: 1px solid var(--line);
      border-radius: 12px;
      background: var(--panel);
      box-shadow: 0 5px 20px rgba(31, 42, 55, 0.06);
      overflow: hidden;
    }
    #graph { width: 100%; height: 100%; }
    .detail {
      width: 320px;
      flex: 0 0 320px;
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 12px;
      box-shadow: 0 5px 20px rgba(31, 42, 55, 0.06);
      overflow: auto;
    }
    .detail-header {
      padding: 12px 14px;
      border-bottom: 1px solid var(--line);
      font-size: 12px;
      letter-spacing: 0.03em;
      text-transform: uppercase;
      font-weight: 700;
      color: var(--muted);
    }
    .detail-body {
      padding: 12px 14px;
      display: flex;
      flex-direction: column;
      gap: 10px;
    }
    .row {
      display: flex;
      flex-direction: column;
      gap: 3px;
      border-bottom: 1px dashed #f0f2f6;
      padding-bottom: 8px;
    }
    .row:last-child { border-bottom: none; }
    .rk {
      color: var(--muted);
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.03em;
      font-weight: 700;
    }
    .rv {
      color: var(--text);
      font-size: 13px;
      word-break: break-word;
    }
    .legend {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      margin-left: 6px;
    }
    .legend-item {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      border: 1px solid var(--line);
      background: #fff;
      border-radius: 999px;
      padding: 4px 8px;
      font-size: 11px;
      color: #4b5563;
      max-width: 220px;
    }
    .legend-dot {
      width: 10px;
      height: 10px;
      border-radius: 50%;
      border: 1px solid rgba(0, 0, 0, 0.12);
      flex: 0 0 auto;
    }
    .filters {
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
      align-items: center;
      margin-top: 4px;
    }
    .filter-group {
      display: flex;
      align-items: center;
      gap: 8px;
      flex-wrap: wrap;
    }
    .filter-title {
      font-size: 11px;
      font-weight: 700;
      color: var(--muted);
      text-transform: uppercase;
      letter-spacing: 0.03em;
    }
    .chips {
      display: inline-flex;
      gap: 6px;
      flex-wrap: wrap;
    }
    .chip {
      border: 1px solid #dbe3ee;
      background: #fff;
      border-radius: 999px;
      padding: 4px 9px;
      font-size: 11px;
      color: #4b5563;
      cursor: pointer;
      user-select: none;
    }
    .chip.active {
      background: #e8f1ff;
      border-color: #7fb1f5;
      color: #1e3a8a;
      font-weight: 700;
    }
    .status {
      position: absolute;
      top: 10px;
      right: 10px;
      z-index: 5;
      padding: 7px 10px;
      border-radius: 999px;
      font-size: 12px;
      font-weight: 600;
      border: 1px solid transparent;
      background: rgba(255, 255, 255, 0.95);
      color: var(--muted);
    }
    .status.ok { color: #065f46; border-color: #a7f3d0; background: #ecfdf5; }
    .status.err { color: #991b1b; border-color: #fecaca; background: #fef2f2; }
    .status.loading { color: #1e40af; border-color: #bfdbfe; background: #eff6ff; }
    @media (max-width: 1100px) {
      .content { flex-direction: column; }
      .detail { width: 100%; flex: 0 0 auto; max-height: 220px; }
    }
  </style>
</head>
<body>
  <div class="app">
    <div class="toolbar">
      <div class="field">
        <label for="startId">start id</label>
        <input id="startId" placeholder="optional node id"/>
      </div>
      <div class="field" style="min-width: 90px;">
        <label for="maxDepth">max depth</label>
        <input id="maxDepth" type="number" min="1" value="2"/>
      </div>
      <div class="field">
        <label for="graphKind">graph kind</label>
        <input id="graphKind" placeholder="knowledge / skill"/>
      </div>
      <div class="field">
        <label for="focusNode">focus node</label>
        <input id="focusNode" placeholder="type node id and press Enter"/>
      </div>
      <div class="actions">
        <button id="toggleLabelBtn" class="btn-ghost">Hide Labels</button>
        <button id="copyLinkBtn" class="btn-ghost">Copy Link</button>
        <button id="fitBtn">Fit</button>
        <button id="stabilizeBtn">Stabilize</button>
        <button id="refreshBtn" class="btn-primary">Refresh</button>
      </div>
    </div>
    <div class="stats">
      <div class="stat"><div class="k">Visible Nodes</div><div class="v" id="visibleNodes">0</div></div>
      <div class="stat"><div class="k">Visible Edges</div><div class="v" id="visibleEdges">0</div></div>
      <div class="stat"><div class="k">Total Nodes</div><div class="v" id="totalNodes">0</div></div>
      <div class="stat"><div class="k">Total Edges</div><div class="v" id="totalEdges">0</div></div>
      <div class="legend" id="legend"></div>
    </div>
    <div class="filters">
      <div class="filter-group">
        <span class="filter-title">Relation Filter</span>
        <div class="chips" id="relationFilters"></div>
      </div>
      <div class="filter-group">
        <span class="filter-title">Node Type Filter</span>
        <div class="chips" id="nodeTypeFilters"></div>
      </div>
      <button id="clearFiltersBtn" class="btn-ghost">Clear Filters</button>
    </div>
    <div class="content">
      <div class="graph-wrap">
        <div id="status" class="status">Ready</div>
        <div id="graph"></div>
      </div>
      <aside class="detail">
        <div class="detail-header" id="detailHeader">Selection</div>
        <div class="detail-body" id="detailBody">
          <div class="row">
            <div class="rk">Hint</div>
            <div class="rv">点击节点或边，查看详细信息（尤其是 edge 的属性）。</div>
          </div>
        </div>
      </aside>
    </div>
  </div>
  <script>
    const statusEl = document.getElementById("status");
    const detailHeader = document.getElementById("detailHeader");
    const detailBody = document.getElementById("detailBody");
    const legendEl = document.getElementById("legend");
    const relationFiltersEl = document.getElementById("relationFilters");
    const nodeTypeFiltersEl = document.getElementById("nodeTypeFilters");
    const nodeMap = new Map();
    const edgeMap = new Map();
    let labelsVisible = true;
    let lastNodes = [];
    let lastEdges = [];
    let rawNodes = [];
    let rawEdges = [];
    const selectedRelationFilters = new Set();
    const selectedNodeTypeFilters = new Set();
    let suppressURLSync = false;

    function setStatus(kind, text) {
      statusEl.className = "status " + kind;
      statusEl.textContent = text;
    }
    function esc(v) {
      if (v === null || v === undefined || v === "") return "-";
      return String(v).replace(/[&<>"']/g, m => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        "\"": "&quot;",
        "'": "&#39;"
      }[m]));
    }
    function hashColor(seed) {
      const s = String(seed || "x");
      let h = 0;
      for (let i = 0; i < s.length; i++) h = ((h << 5) - h) + s.charCodeAt(i);
      const hue = Math.abs(h) % 360;
      return {
        line: "hsl(" + hue + ", 65%, 56%)",
        pale: "hsla(" + hue + ", 70%, 85%, 0.65)",
      };
    }
    function setDetail(title, rows) {
      detailHeader.textContent = title;
      detailBody.innerHTML = rows.map(([k, v]) =>
        "<div class='row'><div class='rk'>" + esc(k) + "</div><div class='rv'>" + esc(v) + "</div></div>"
      ).join("");
    }
    function nodeColorByType(nodeType) {
      const c = hashColor(nodeType || "node");
      return {
        background: c.pale.replace("0.65", "0.92"),
        border: c.line,
        highlight: { background: c.pale.replace("0.65", "1"), border: c.line }
      };
    }
    function renderLegend(items) {
      legendEl.innerHTML = items.slice(0, 8).map((it) =>
        "<span class='legend-item'><span class='legend-dot' style='background:" + esc(it.color) + ";border-color:" + esc(it.color) + "'></span>" +
        "<span>" + esc(it.label) + "</span></span>"
      ).join("");
    }
    function relationKey(edge) {
      return edge.relation_type || edge.graph_kind || "edge";
    }
    function nodeTypeKey(node) {
      return node.node_type || "unknown";
    }
    function renderFilterChips(container, values, selectedSet) {
      container.innerHTML = "";
      values.slice(0, 24).forEach(v => {
        const chip = document.createElement("button");
        chip.type = "button";
        chip.className = "chip" + (selectedSet.has(v) ? " active" : "");
        chip.dataset.value = v;
        chip.textContent = v;
        container.appendChild(chip);
      });
    }
    function syncURLState() {
      if (suppressURLSync) return;
      const u = new URL(window.location.href);
      const p = u.searchParams;
      const startId = document.getElementById("startId").value.trim();
      const maxDepth = document.getElementById("maxDepth").value.trim();
      const graphKind = document.getElementById("graphKind").value.trim();
      const focusNode = document.getElementById("focusNode").value.trim();
      if (startId) p.set("start_id", startId); else p.delete("start_id");
      if (maxDepth) p.set("max_depth", maxDepth); else p.delete("max_depth");
      if (graphKind) p.set("graph_kind", graphKind); else p.delete("graph_kind");
      if (focusNode) p.set("focus_node", focusNode); else p.delete("focus_node");
      if (!labelsVisible) p.set("labels", "off"); else p.delete("labels");
      const rel = Array.from(selectedRelationFilters);
      const nts = Array.from(selectedNodeTypeFilters);
      if (rel.length > 0) p.set("rel_filters", rel.join(","));
      else p.delete("rel_filters");
      if (nts.length > 0) p.set("node_type_filters", nts.join(","));
      else p.delete("node_type_filters");
      window.history.replaceState({}, "", u.toString());
    }
    function parseCSVParam(value) {
      return (value || "")
        .split(",")
        .map(s => s.trim())
        .filter(Boolean);
    }
    function restoreStateFromURL() {
      suppressURLSync = true;
      const p = new URLSearchParams(window.location.search);
      const startId = p.get("start_id") || "";
      const maxDepth = p.get("max_depth") || "2";
      const graphKind = p.get("graph_kind") || "";
      const focusNode = p.get("focus_node") || "";
      document.getElementById("startId").value = startId;
      document.getElementById("maxDepth").value = maxDepth;
      document.getElementById("graphKind").value = graphKind;
      document.getElementById("focusNode").value = focusNode;
      labelsVisible = (p.get("labels") || "").toLowerCase() !== "off";
      document.getElementById("toggleLabelBtn").textContent = labelsVisible ? "Hide Labels" : "Show Labels";
      selectedRelationFilters.clear();
      selectedNodeTypeFilters.clear();
      parseCSVParam(p.get("rel_filters")).forEach(v => selectedRelationFilters.add(v));
      parseCSVParam(p.get("node_type_filters")).forEach(v => selectedNodeTypeFilters.add(v));
      suppressURLSync = false;
    }
    function buildVisData(nodesRaw, edgesRaw) {
      const nodes = nodesRaw.map(n => {
        nodeMap.set(n.id, n);
        return {
          id: n.id,
          label: n.id,
          color: nodeColorByType(n.node_type),
          title:
            "id: " + (n.id || "") + "\n" +
            "type: " + (n.node_type || "") + "\n" +
            "status: " + (n.status || "")
        };
      });
      const edges = edgesRaw.map(e => {
        edgeMap.set(e.id, e);
        const c = hashColor(relationKey(e));
        return {
          id: e.id,
          from: e.from_id,
          to: e.to_id,
          label: e.relation_type || "",
          color: { color: c.line, highlight: c.line, hover: c.line, opacity: 0.85 },
          title:
            "relation: " + (e.relation_type || "") + "\n" +
            "kind: " + (e.graph_kind || "") + "\n" +
            "confidence: " + (e.confidence ?? "") + "\n" +
            "condition: " + (e.condition_text || "") + "\n" +
            "evidence: " + (e.evidence_count ?? 0) + ", failed: " + (e.failed_count ?? 0)
        };
      });
      return { nodes, edges };
    }
    function applyClientFilters() {
      let filteredNodes = rawNodes;
      if (selectedNodeTypeFilters.size > 0) {
        filteredNodes = rawNodes.filter(n => selectedNodeTypeFilters.has(nodeTypeKey(n)));
      }
      const visibleNodeIDs = new Set(filteredNodes.map(n => n.id));
      let filteredEdges = rawEdges.filter(e => visibleNodeIDs.has(e.from_id) && visibleNodeIDs.has(e.to_id));
      if (selectedRelationFilters.size > 0) {
        filteredEdges = filteredEdges.filter(e => selectedRelationFilters.has(relationKey(e)));
      }
      const vis = buildVisData(filteredNodes, filteredEdges);
      applyGraphData(vis.nodes, vis.edges);
      updateStats(filteredNodes.length, filteredEdges.length, rawNodes.length, rawEdges.length);
      syncURLState();
    }
    function applyGraphData(nodes, edges) {
      const nextNodes = nodes.map(n => ({ ...n }));
      const nextEdges = edges.map(e => ({ ...e, label: labelsVisible ? (e.label || "") : "" }));
      network.setData({ nodes: nextNodes, edges: nextEdges });
      lastNodes = nodes;
      lastEdges = edges;
    }

    const container = document.getElementById("graph");
    const network = new vis.Network(container, {nodes: [], edges: []}, {
      interaction: {
        hover: true,
        tooltipDelay: 120,
        zoomView: true,
        dragView: true
      },
      nodes: {
        shape: "dot",
        size: 16,
        shadow: { enabled: true, color: "rgba(31,42,55,0.18)", size: 8, x: 0, y: 3 },
        font: {
          size: 14,
          color: "#334155",
          face: "Inter, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
        },
        borderWidth: 2
      },
      edges: {
        arrows: { to: { enabled: true, scaleFactor: 0.75 } },
        smooth: { enabled: true, type: "curvedCW", roundness: 0.18 },
        font: { size: 12, color: "#64748b", strokeWidth: 3, strokeColor: "rgba(255,255,255,0.9)" },
        width: 2,
        selectionWidth: 3
      },
      physics: {
        stabilization: { enabled: true, iterations: 160 },
        barnesHut: { springLength: 130, springConstant: 0.02, damping: 0.74, avoidOverlap: 0.46 }
      }
    });

    function updateStats(nodes, edges, totalNodes, totalEdges) {
      document.getElementById("visibleNodes").textContent = String(nodes);
      document.getElementById("visibleEdges").textContent = String(edges);
      document.getElementById("totalNodes").textContent = String(totalNodes);
      document.getElementById("totalEdges").textContent = String(totalEdges);
    }

    async function refresh() {
      const startId = document.getElementById("startId").value.trim();
      const maxDepth = document.getElementById("maxDepth").value.trim();
      const graphKind = document.getElementById("graphKind").value.trim();
      const q = new URLSearchParams();
      if (startId) q.set("start_id", startId);
      if (maxDepth) q.set("max_depth", maxDepth);
      if (graphKind) q.set("graph_kind", graphKind);
      syncURLState();

      setStatus("loading", "Loading...");
      try {
        const res = await fetch("/api/graph?" + q.toString());
        const data = await res.json();
        if (!res.ok) {
          setStatus("err", data.error || "Failed to load graph");
          return;
        }
        nodeMap.clear();
        edgeMap.clear();
        rawNodes = Array.isArray(data.nodes) ? data.nodes : [];
        rawEdges = Array.isArray(data.edges) ? data.edges : [];
        const relationOptions = Array.from(new Set(rawEdges.map(relationKey))).sort();
        const nodeTypeOptions = Array.from(new Set(rawNodes.map(nodeTypeKey))).sort();
        renderFilterChips(relationFiltersEl, relationOptions, selectedRelationFilters);
        renderFilterChips(nodeTypeFiltersEl, nodeTypeOptions, selectedNodeTypeFilters);
        applyClientFilters();
        const relationColors = new Map();
        rawEdges.forEach(e => {
          const key = relationKey(e);
          if (!relationColors.has(key)) {
            relationColors.set(key, hashColor(key).line);
          }
        });
        renderLegend(Array.from(relationColors.entries()).map(([label, color]) => ({ label, color })));
        if (lastNodes.length > 0) {
          network.fit({ animation: { duration: 260, easingFunction: "easeInOutQuad" } });
        }
        setDetail("Selection", [
          ["Hint", "点击节点或边，查看详细信息（尤其是 edge 的属性）。"],
          ["Current Filter", "start_id=" + (startId || "-") + ", max_depth=" + (maxDepth || "-") + ", graph_kind=" + (graphKind || "-")]
        ]);
        setStatus("ok", "Loaded");
      } catch (err) {
        setStatus("err", "Network error");
      }
    }

    document.getElementById("refreshBtn").addEventListener("click", refresh);
    document.getElementById("toggleLabelBtn").addEventListener("click", () => {
      labelsVisible = !labelsVisible;
      document.getElementById("toggleLabelBtn").textContent = labelsVisible ? "Hide Labels" : "Show Labels";
      if (lastNodes.length > 0 || lastEdges.length > 0) {
        applyGraphData(lastNodes, lastEdges);
      }
      syncURLState();
    });
    relationFiltersEl.addEventListener("click", (ev) => {
      const btn = ev.target.closest(".chip");
      if (!btn) return;
      const value = btn.dataset.value || "";
      if (!value) return;
      if (selectedRelationFilters.has(value)) selectedRelationFilters.delete(value);
      else selectedRelationFilters.add(value);
      renderFilterChips(relationFiltersEl, Array.from(new Set(rawEdges.map(relationKey))).sort(), selectedRelationFilters);
      applyClientFilters();
    });
    nodeTypeFiltersEl.addEventListener("click", (ev) => {
      const btn = ev.target.closest(".chip");
      if (!btn) return;
      const value = btn.dataset.value || "";
      if (!value) return;
      if (selectedNodeTypeFilters.has(value)) selectedNodeTypeFilters.delete(value);
      else selectedNodeTypeFilters.add(value);
      renderFilterChips(nodeTypeFiltersEl, Array.from(new Set(rawNodes.map(nodeTypeKey))).sort(), selectedNodeTypeFilters);
      applyClientFilters();
    });
    document.getElementById("clearFiltersBtn").addEventListener("click", () => {
      selectedRelationFilters.clear();
      selectedNodeTypeFilters.clear();
      renderFilterChips(relationFiltersEl, Array.from(new Set(rawEdges.map(relationKey))).sort(), selectedRelationFilters);
      renderFilterChips(nodeTypeFiltersEl, Array.from(new Set(rawNodes.map(nodeTypeKey))).sort(), selectedNodeTypeFilters);
      applyClientFilters();
      setStatus("ok", "Filters cleared");
    });
    document.getElementById("copyLinkBtn").addEventListener("click", async () => {
      syncURLState();
      try {
        await navigator.clipboard.writeText(window.location.href);
        setStatus("ok", "Link copied");
      } catch (e) {
        setStatus("err", "Copy failed");
      }
    });
    ["startId", "maxDepth", "graphKind", "focusNode"].forEach(id => {
      document.getElementById(id).addEventListener("change", syncURLState);
    });
    document.getElementById("fitBtn").addEventListener("click", () => {
      network.fit({ animation: { duration: 240, easingFunction: "easeInOutQuad" } });
    });
    document.getElementById("stabilizeBtn").addEventListener("click", () => {
      setStatus("loading", "Stabilizing...");
      network.stabilize(120);
      setStatus("ok", "Loaded");
    });
    network.on("selectNode", params => {
      const id = params.nodes && params.nodes[0];
      const n = nodeMap.get(id);
      if (!n) return;
      setDetail("Node", [
        ["id", n.id],
        ["type", n.node_type],
        ["status", n.status],
        ["aliases", Array.isArray(n.aliases) ? n.aliases.join(", ") : n.aliases],
        ["source", n.source],
      ]);
    });
    network.on("selectEdge", params => {
      const id = params.edges && params.edges[0];
      const e = edgeMap.get(id);
      if (!e) return;
      setDetail("Edge", [
        ["id", e.id],
        ["from", e.from_id],
        ["to", e.to_id],
        ["relation", e.relation_type],
        ["graph_kind", e.graph_kind],
        ["confidence", e.confidence],
        ["condition_text", e.condition_text],
        ["evidence_count", e.evidence_count],
        ["failed_count", e.failed_count],
        ["observed_at", e.observed_at],
        ["valid_from", e.valid_from],
        ["valid_until", e.valid_until],
      ]);
    });
    network.on("deselectNode", () => {
      setDetail("Selection", [["Hint", "点击边可以查看 edge 的完整属性。"]]);
    });
    network.on("deselectEdge", () => {
      setDetail("Selection", [["Hint", "点击节点或边查看详情。"]]);
    });
    document.getElementById("focusNode").addEventListener("keydown", (ev) => {
      if (ev.key !== "Enter") return;
      const id = ev.target.value.trim();
      if (!id) return;
      const n = nodeMap.get(id);
      if (!n) {
        setStatus("err", "Node not found: " + id);
        return;
      }
      network.selectNodes([id]);
      network.focus(id, { scale: 1.2, animation: { duration: 260, easingFunction: "easeInOutQuad" } });
      setDetail("Node", [
        ["id", n.id],
        ["type", n.node_type],
        ["status", n.status],
        ["aliases", Array.isArray(n.aliases) ? n.aliases.join(", ") : n.aliases],
        ["source", n.source],
      ]);
      setStatus("ok", "Focused: " + id);
    });
    restoreStateFromURL();
    refresh();
  </script>
</body>
</html>`
