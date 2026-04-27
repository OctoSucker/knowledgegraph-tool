APP := kgtool
GOCACHE ?= $(CURDIR)/.gocache

.PHONY: fmt test build install smoke release-snapshot npm-pack sync-npm-version

fmt:
	GOCACHE=$(GOCACHE) gofmt -w .

test:
	GOCACHE=$(GOCACHE) go test ./...

build:
	mkdir -p bin
	GOCACHE=$(GOCACHE) go build -o bin/$(APP) ./cmd/kgtool

install:
	GOCACHE=$(GOCACHE) go install ./cmd/kgtool

smoke:
	rm -f /tmp/knowledgegraph-tool-smoke.sqlite
	GOCACHE=$(GOCACHE) go run ./cmd/kgtool add-edge --db /tmp/knowledgegraph-tool-smoke.sqlite --from-id alpha --to-id beta
	GOCACHE=$(GOCACHE) go run ./cmd/kgtool list-edges --db /tmp/knowledgegraph-tool-smoke.sqlite

release-snapshot:
	GOCACHE=$(GOCACHE) goreleaser release --snapshot --clean

npm-pack:
	cd npm && npm pack

sync-npm-version:
	node scripts/set-npm-version.mjs $(VERSION)
