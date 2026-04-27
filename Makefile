APP := kggraph
GOCACHE ?= $(CURDIR)/.gocache

.PHONY: fmt test build install smoke

fmt:
	GOCACHE=$(GOCACHE) gofmt -w .

test:
	GOCACHE=$(GOCACHE) go test ./...

build:
	mkdir -p bin
	GOCACHE=$(GOCACHE) go build -o bin/$(APP) ./cmd/kggraph

install:
	GOCACHE=$(GOCACHE) go install ./cmd/kggraph

smoke:
	rm -f /tmp/kggraph-smoke.sqlite
	GOCACHE=$(GOCACHE) go run ./cmd/kggraph add-edge --db /tmp/kggraph-smoke.sqlite --from-id alpha --to-id beta
	GOCACHE=$(GOCACHE) go run ./cmd/kggraph list-edges --db /tmp/kggraph-smoke.sqlite
