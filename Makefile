.PHONY: all build proto web go test test-integration run-simple clean

all: build

# --- Proto ---
proto:
	buf generate

# --- Web ---
web: proto
	cd web && npm install && npm run build

# --- Go binary (includes embedded frontend) ---
go: web
	go build -o bin/norn ./cmd/norn

build: go

# --- Tests ---
test:
	@mkdir -p web/dist && touch web/dist/placeholder
	go test ./...

test-integration:
	go test -tags integration -v -timeout 10m ./tests/integration/

# --- Dev ---
dev-web:
	cd web && npm run dev

dev-go:
	go run ./cmd/norn

# --- Run ---
run-simple: go
	./bin/norn --workspace-folder examples/simple --storage-dir .norn

# --- Clean ---
clean:
	rm -rf bin/ web/dist web/node_modules/.tmp
