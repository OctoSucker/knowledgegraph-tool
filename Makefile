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
	OPENAI_API_KEY= OPENAI_BASE_URL= OPENAI_EMBEDDING_MODEL= OPENAI_MODEL= GOCACHE=$(GOCACHE) go run ./cmd/kggraph add-fact-edge --db /tmp/kggraph-smoke.sqlite --from-id alpha --to-id beta --relation-type related_to --confidence 0.7
	OPENAI_API_KEY= OPENAI_BASE_URL= OPENAI_EMBEDDING_MODEL= OPENAI_MODEL= GOCACHE=$(GOCACHE) go run ./cmd/kggraph list-edges --db /tmp/kggraph-smoke.sqlite
