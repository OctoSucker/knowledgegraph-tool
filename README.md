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

Homebrew:

```bash
brew install --formula https://raw.githubusercontent.com/OctoSucker/KGgraph/main/Formula/kggraph.rb
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

This repository is structured to be used as a Homebrew formula source.

Release flow:

1. Create a source tag
2. Update `url` and `sha256` in [Formula/kggraph.rb](/Users/zecrey/Desktop/yiming/KGgraph/Formula/kggraph.rb)
3. Commit the formula update
4. Either install directly from the raw formula URL or sync the formula into a dedicated Homebrew tap repository

The Homebrew formula builds `kggraph` from source.
