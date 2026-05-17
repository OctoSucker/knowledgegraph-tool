# KGgraph

`KGgraph` is a **lightweight graph memory and reasoning expansion tool** for AI agents and humans.

Use it to:
- store relationships as a graph
- expand weighted multi-hop paths (`A -> B -> C`)
- limit noisy expansion with depth, branch, score, and time filters

It provides:
- **CLI** (`kggraph ...`)
- **MCP stdio server** (`kggraph serve-mcp`)
- **Local graph viewer** (`kggraph graph-view`)

You can start with plain language input. KGgraph will extract nodes/edges and write them for you.

## Install

Homebrew (recommended):

```bash
brew tap 0xfakeSpike/tap
brew install kggraph
```

## Quick start (30 seconds)

```bash
# 1) Ingest one statement (recommended)
kggraph ingest-statement \
  --workspace ./workspace \
  --statement "战争升级通常会推高原油价格，并在市场未提前消化时压制美股大盘"

# 2) Expand reasoning from one node
kggraph expand-reasoning \
  --workspace ./workspace \
  --start-id "战争升级" \
  --max-depth 3
```

## Manual mode (optional)

If you want explicit control, add nodes/edges directly:

```bash
kggraph upsert-node --workspace ./workspace --id "战争升级" --node-type event
kggraph upsert-node --workspace ./workspace --id "原油上涨" --node-type event
kggraph add-fact-edge --workspace ./workspace --from-id "战争升级" --to-id "原油上涨" --relation-type increases_probability_of --confidence 0.72
```

## Defaults (no extra setup needed)

By default, KGgraph auto-fills edge time fields internally:
- `observed_at = now`
- `valid_from = observed_at`
- `valid_until = null` (open-ended)
- reasoning `as_of = now`

You only need to provide extra time fields when you want strict time-window behavior.

For `ingest-statement`, KGgraph asks the LLM to infer edge time fields from the statement first; internal defaults are used only when the model cannot infer them.

## MCP usage

```bash
kggraph serve-mcp --workspace ./workspace
```

Main tools:
- `kg_upsert_node`
- `kg_add_fact_edge`
- `kg_add_skill_edge`
- `kg_ingest_statement`
- `kg_expand_reasoning`
- `kg_lookup_node_exact`
- `kg_lookup_node_semantic`
- `kg_list_nodes`
- `kg_list_edges`
- `kg_attach_edge_evidence`
- `kg_verify_edge`

Example MCP client config: `examples/mcp-stdio.json`

## Graph viewer (manual refresh)

```bash
kggraph graph-view
# open http://127.0.0.1:8787
```

In the viewer, set `start-id` / `max-depth` / `graph-kind`, then click `Refresh` to reload from SQLite.

## Limitations

- not a full logical reasoner
- LLM ingestion can still create noisy nodes/edges
- semantic lookup requires embeddings
- SQLite target is local/small-to-medium agent memory

## Data location

DB path resolution order:
1. `--db /path/to/knowledgegraph.sqlite`
2. `KG_DB_PATH`
3. `--workspace WORKSPACE` -> `WORKSPACE/data/knowledgegraph.sqlite`
4. User data directory:
   - macOS: `~/Library/Application Support/kggraph/knowledgegraph.sqlite`
   - Linux: `${XDG_DATA_HOME}/kggraph/knowledgegraph.sqlite` or `~/.local/share/kggraph/knowledgegraph.sqlite`
   - Windows: `%APPDATA%\kggraph\knowledgegraph.sqlite`

For agent or MCP usage, prefer `--workspace` or `--db` to avoid mixing unrelated projects in one graph.
