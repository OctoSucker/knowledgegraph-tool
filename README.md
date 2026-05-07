# KGgraph

`KGgraph` is a standalone knowledge graph utility for agents and automation systems.

It exposes the same capability surface through:

- a CLI binary: `kggraph`
- an MCP stdio server: `kggraph serve-mcp`

It stores graph data in SQLite and optionally uses an OpenAI-compatible embedding model for semantic lookup.

## Features

- Directed graph storage with signed edges
- Exact node lookup
- Semantic node lookup with embeddings
- Batch edge insertion
- CLI and MCP parity for all exported tools

## Installation

Homebrew ([tap](https://github.com/0xfakeSpike/homebrew-tap)):

```bash
brew tap 0xfakeSpike/tap
brew install kggraph
```

Build locally:

```bash
make build
```

Go install:

```bash
go install github.com/OctoSucker/KGgraph/cmd/kggraph@latest
```

## CLI

List nodes:

```bash
kggraph list-nodes --workspace ./workspace
```

Add an edge:

```bash
kggraph add-edge --workspace ./workspace --from-id alpha --to-id beta
```

Generic tool call:

```bash
kggraph call \
  --workspace ./workspace \
  --tool kg_add_edge \
  --args-json '{"from_id":"alpha","to_id":"beta","positive":true}'
```

Semantic lookup:

```bash
kggraph lookup-node-semantic \
  --workspace ./workspace \
  --term "NASDAQ" \
  --api-key "$OPENAI_API_KEY" \
  --embedding-model text-embedding-3-small
```

## MCP

Start MCP over stdio:

```bash
kggraph serve-mcp \
  --workspace ./workspace \
  --api-key "$OPENAI_API_KEY" \
  --embedding-model text-embedding-3-small
```

Exposed MCP tools:

- `kg_add_edge`
- `kg_add_edges_batch`
- `kg_lookup_node_exact`
- `kg_lookup_node_semantic`
- `kg_list_nodes`
- `kg_list_edges`

Example client config: [examples/mcp-stdio.json](/Users/zecrey/Desktop/yiming/KGgraph/examples/mcp-stdio.json)

## Development

```bash
make fmt
make test
make build
make smoke
```

## Storage

Default database path:

- `WORKSPACE/data/knowledgegraph.sqlite`

Or pass an explicit SQLite file with `--db`.

## Homebrew Publishing

The formula lives in [0xfakeSpike/homebrew-tap](https://github.com/0xfakeSpike/homebrew-tap/blob/main/kggraph.rb).

Release flow:

1. Create a source tag on this repository
2. Update `url` and `sha256` in `kggraph.rb` in the tap repository
3. Commit and push the tap change

The formula builds `kggraph` from source.
