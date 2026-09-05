.PHONY: all build build-web build-core build-agent test clean docker-build up down

VERSION ?= 0.1.0-mvp
BUILD_DIR ?= ./bin

all: build

build-web:
	@echo "==> Building web frontend..."
	cd web && npm install && npm run build

build-core:
	@echo "==> Building panel-core binary..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(BUILD_DIR)/panel-core ./cmd/panel-core

build-agent:
	@echo "==> Building panel-agent binary..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(BUILD_DIR)/panel-agent ./cmd/panel-agent

build: build-web build-core build-agent

test:
	@echo "==> Running backend unit & integration tests..."
	go test -v -race -cover ./...

clean:
	@echo "==> Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf cmd/panel-core/webdist

docker-build:
	@echo "==> Building Docker images..."
	docker compose build

up:
	@echo "==> Starting Docker Compose Panel..."
	docker compose up -d

down:
	@echo "==> Stopping Docker Compose Panel..."
	docker compose down
