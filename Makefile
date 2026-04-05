.PHONY: all build proto web go test test-e2e test-e2e-server test-e2e-daemon run-simple clean

all: build

# --- Proto ---
proto:
	buf generate

# --- Web ---
web: proto
	cd web && npm install && npm run build

# --- Go binary (includes embedded frontend) ---
go: web
	CGO_ENABLED=0 go build -o bin/norn ./cmd/norn

docker: go
	docker compose build

build: go docker

# --- Tests ---
test:
	@mkdir -p web/dist && touch web/dist/placeholder
	go test ./...

test-e2e:
	go test -v -timeout 15m ./tests/e2e/...

test-e2e-daemon:
	go test -v -timeout 10m ./tests/e2e/daemon/

test-e2e-server:
	go test -v -timeout 15m ./tests/e2e/server/

# --- Dev ---
dev-web:
	cd web && npm run dev

dev-go:
	go run ./cmd/norn

# --- Run ---
run-simple: build
	./bin/norn server --workspace-folder examples/simple --storage-dir .norn

# --- Clean ---
clean:
	rm -rf bin/ web/dist web/node_modules/.tmp
