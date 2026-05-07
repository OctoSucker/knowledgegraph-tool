# KGgraph

`KGgraph` is a **lightweight graph memory and reasoning expansion tool** for AI agents and humans.

Use it to:
- store relationships as a graph
- expand weighted multi-hop paths (`A -> B -> C`)
- limit noisy expansion with depth, branch, score, and time filters

It provides:
- **CLI** (`kggraph ...`)
- **MCP stdio server** (`kggraph serve-mcp`)

You can start with plain language input. KGgraph will extract nodes/edges and write them for you.

## Install

```bash
go install github.com/OctoSucker/KGgraph/cmd/kggraph@latest
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

## Limitations

- not a full logical reasoner
- LLM ingestion can still create noisy nodes/edges
- semantic lookup requires embeddings
- SQLite target is local/small-to-medium agent memory

## Data location

Default DB:
- `WORKSPACE/data/knowledgegraph.sqlite`

Or set:
- `--db /path/to/knowledgegraph.sqlite`
