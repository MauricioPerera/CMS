# Makefile C64 — build + deploy de gopress-server.
# CGO_ENABLED=1 requerido (mattn/go-sqlite3 + github.com/buke/quickjs-go).

GO ?= go
BUILD_DIR ?= bin
BINARY = $(BUILD_DIR)/gopress-server

.PHONY: all build build-linux build-windows test smoke clean deploy-deploy deploy-dev

all: build

build:
	$(GO) build -o $(BINARY) ./cmd/server

# Build para Linux/amd64 (target Worker sandbox).
build-linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/gopress-server-linux ./cmd/server

build-windows:
	$(GO) build -o $(BINARY).exe ./cmd/server

test:
	$(GO) vet ./...
	$(GO) test -race -count=1 ./internal/...

# Smoke test local: build + arranca + healthz + posts.
smoke: build
	PORT=8199 DB_PATH=":memory:" MIGRATIONS_DIR=db/migrations \
	  timeout 8s $(BINARY) & \
	  sleep 2 && curl -sf http://127.0.0.1:8199/healthz && curl -sf http://127.0.0.1:8199/posts ; kill %1 2>/dev/null ; true

clean:
	rm -rf $(BUILD_DIR)

# Deploy (requiere wrangler + node + CF_ACCOUNT_ID).
deploy-deploy:
	wrangler deploy --env production

deploy-dev:
	wrangler dev
