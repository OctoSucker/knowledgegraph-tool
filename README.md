# knowledgegraph-tool

`knowledgegraph-tool` is a standalone knowledge graph utility for agents and automation systems.

It provides two integration modes:

- CLI
- MCP stdio server

It stores graph data in SQLite and optionally uses OpenAI-compatible embeddings for semantic lookup.

The intended contract is:

- exact/list operations work offline
- semantic lookup requires an embedding model
- every capability is available from both CLI and MCP

## Features

- Directed graph storage: nodes and signed edges
- Exact node lookup
- Semantic node lookup with embeddings
- Batch edge insertion
- CLI-first invocation model
- MCP tools for agent integration

## Installation

```bash
go build ./cmd/kgtool
```

Or:

```bash
make build
```

Go users can also install directly:

```bash
go install github.com/OctoSucker/knowledgegraph-tool/cmd/kgtool@latest
```

If you want one-command installation from the Node ecosystem, use the npm wrapper:

```bash
npm install -g knowledgegraph-tool
```

The npm package downloads the matching prebuilt `kgtool` binary from GitHub Releases.

For release publishing, this repository includes:

- [`.goreleaser.yaml`](/Users/zecrey/Desktop/yiming/knowledgegraph-tool/.goreleaser.yaml) for multi-platform binaries
- [npm/package.json](/Users/zecrey/Desktop/yiming/knowledgegraph-tool/npm/package.json) as the npm wrapper package

## CLI

List nodes:

```bash
go run ./cmd/kgtool list-nodes --workspace ./workspace
```

Add an edge:

```bash
go run ./cmd/kgtool add-edge --workspace ./workspace --from-id alpha --to-id beta
```

Generic tool call:

```bash
go run ./cmd/kgtool call \
  --workspace ./workspace \
  --tool kg_add_edge \
  --args-json '{"from_id":"alpha","to_id":"beta","positive":true}'
```

Semantic lookup:

```bash
go run ./cmd/kgtool lookup-node-semantic \
  --workspace ./workspace \
  --term "NASDAQ" \
  --api-key "$OPENAI_API_KEY" \
  --embedding-model text-embedding-3-small
```

## MCP

Start MCP over stdio:

```bash
go run ./cmd/kgtool serve-mcp \
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

An MCP client config example is provided at [examples/mcp-stdio.json](/Users/zecrey/Desktop/yiming/knowledgegraph-tool/examples/mcp-stdio.json).

## Development

Useful commands:

```bash
make fmt
make test
make build
make smoke
make release-snapshot
make npm-pack
```

## Storage

By default the tool stores data at:

- `WORKSPACE/data/knowledgegraph.sqlite`

You can also pass an explicit SQLite file path with `--db`.

## Notes

- Exact operations and list operations work without OpenAI credentials.
- Semantic lookup requires an OpenAI-compatible embedding endpoint.
- The default database path is `WORKSPACE/data/knowledgegraph.sqlite`.

## Release Model

Recommended distribution flow:

1. Publish GitHub Releases with `goreleaser`
2. Publish the `npm/` package to npm
3. Let npm users install the prebuilt binary with `npm install -g knowledgegraph-tool`

The npm installer resolves the current OS/CPU and downloads the matching release artifact.

## Publishing

This repository is now set up for local manual publishing.

Recommended flow:

1. Run tests locally
2. Build release artifacts locally with `goreleaser`
3. Publish the npm wrapper locally from `npm/`

Example:

```bash
make test
goreleaser release --clean
cd npm
npm publish --access public
```

Manual preconditions:

- The GitHub repository URL in `npm/package.json` must match the real repo
- The module path in `go.mod` must match the real repo path
- The npm package name must still be available
- You must already be authenticated locally with npm
