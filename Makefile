export PATH := $(PATH):$(HOME)/go/bin
WAILS_TAGS := webkit2_41

.PHONY: dev run build backend install-launcher test lint typecheck frontend-deps frontend-assets

dev run:
	cd wails-app && RERUM_DB_PATH="$${RERUM_DB_PATH:-$$(mktemp -d /tmp/rerum-dev.XXXXXX)/rerum.sqlite}" wails dev -tags $(WAILS_TAGS)

build:
	cd wails-app/frontend && npm install --legacy-peer-deps && npm run build
	cd wails-app && wails build -s -tags $(WAILS_TAGS)

backend:
	cd backend && go run ./cmd/rerum-backend --db "$${RERUM_DB_PATH:-$$(mktemp -d /tmp/rerum-backend.XXXXXX)/rerum.sqlite}" --listen 127.0.0.1:3333

install-launcher:
	bash scripts/install-desktop-launcher.sh

frontend-deps:
	cd wails-app/frontend && { test -x node_modules/.bin/vite && test -x node_modules/.bin/svelte-check || npm ci --legacy-peer-deps; }

frontend-assets: frontend-deps
	@test -f wails-app/frontend/dist/index.html || npm --prefix wails-app/frontend run build

test: frontend-assets
	cd backend && go test ./...
	cd wails-app && go test -tags $(WAILS_TAGS) ./...

lint: frontend-assets
	@test -z "$$(gofmt -l backend wails-app)" || (gofmt -l backend wails-app; echo "gofmt: arquivos fora do padrão"; exit 1)
	cd backend && go vet ./...
	cd wails-app && go vet -tags $(WAILS_TAGS) ./...

typecheck: frontend-deps
	cd wails-app/frontend && npm run check
