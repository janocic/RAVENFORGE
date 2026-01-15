.PHONY: all build install clean test docker help

# Variables
VERSION := 1.0.0
PREFIX := /usr/local
CONFIG_DIR := /etc/ravenforge
DATA_DIR := /var/lib/ravenforge
LOG_DIR := /var/log/ravenforge
GO := go
GOFLAGS := -trimpath

# Detect OS
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
    CGO_ENABLED := 1
endif
ifeq ($(UNAME_S),Darwin)
    CGO_ENABLED := 1
endif

# Binary names
DAEMON := ravenforged
CLI := ravenforge
ifeq ($(OS),Windows_NT)
    DAEMON := ravenforged.exe
    CLI := ravenforge.exe
endif

all: build

## Build targets
build: build-daemon build-cli

build-daemon:
	@echo "Building $(DAEMON)..."
	cd core && CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -o $(DAEMON) ./cmd/ravenforged

build-cli:
	@echo "Building $(CLI)..."
	cd core && CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -o $(CLI) ./cmd/ravenforge

## Install targets
install: build install-bin install-config install-service install-tools
	@echo "Installation complete!"

install-bin:
	@echo "Installing binaries..."
	install -Dm755 core/$(DAEMON) $(PREFIX)/bin/$(DAEMON)
	install -Dm755 core/$(CLI) $(PREFIX)/bin/$(CLI)

install-config:
	@echo "Installing configuration..."
	install -dm755 $(CONFIG_DIR)
	install -dm755 $(CONFIG_DIR)/policies
	@if [ ! -f $(CONFIG_DIR)/ravenforge.yaml ]; then \
		install -Dm644 core/config/ravenforge.linux.yaml $(CONFIG_DIR)/ravenforge.yaml; \
	fi

install-service:
	@echo "Installing systemd service..."
	install -Dm644 scripts/ravenforged.service /etc/systemd/system/ravenforged.service
	systemctl daemon-reload

install-tools:
	@echo "Installing tool manifests..."
	install -dm755 $(DATA_DIR)/tools
	cp -r tools/* $(DATA_DIR)/tools/

install-dirs:
	@echo "Creating directories..."
	install -dm750 $(DATA_DIR)
	install -dm750 $(DATA_DIR)/artifacts
	install -dm750 $(LOG_DIR)

## Docker targets
docker: docker-build

docker-build:
	@echo "Building Docker images..."
	docker build -t ravenforge/ingest-jsonlines:$(VERSION) -f tools/ingest/ingest-jsonlines/Dockerfile .
	docker build -t ravenforge/detect-simple-rules:$(VERSION) -f tools/detect/detect-simple-rules/Dockerfile .
	docker build -t ravenforge/enrich-geoip:$(VERSION) -f tools/enrich/enrich-geoip/Dockerfile .
	docker build -t ravenforge/correlate-events:$(VERSION) -f tools/correlate/correlate-events/Dockerfile .
	docker build -t ravenforge/report-generate:$(VERSION) -f tools/report/report-generate/Dockerfile .
	docker build -t ravenforge/triage-prioritize:$(VERSION) -f tools/triage/triage-prioritize/Dockerfile .

docker-push:
	@echo "Pushing Docker images..."
	docker push ravenforge/ingest-jsonlines:$(VERSION)
	docker push ravenforge/detect-simple-rules:$(VERSION)
	docker push ravenforge/enrich-geoip:$(VERSION)
	docker push ravenforge/correlate-events:$(VERSION)
	docker push ravenforge/report-generate:$(VERSION)
	docker push ravenforge/triage-prioritize:$(VERSION)

## Test targets
test:
	@echo "Running tests..."
	cd core && $(GO) test -v ./...

test-short:
	@echo "Running short tests..."
	cd core && $(GO) test -short ./...

test-coverage:
	@echo "Running tests with coverage..."
	cd core && $(GO) test -cover -coverprofile=coverage.out ./...
	cd core && $(GO) tool cover -html=coverage.out -o coverage.html

## Clean targets
clean:
	@echo "Cleaning..."
	rm -f core/$(DAEMON) core/$(CLI)
	rm -f core/coverage.out core/coverage.html
	cd core && $(GO) clean

## Development targets
dev: build
	./core/$(DAEMON) --config core/config/ravenforge.linux.yaml --log-format console

deps:
	@echo "Installing dependencies..."
	cd core && $(GO) mod download
	cd core && $(GO) mod tidy

fmt:
	@echo "Formatting code..."
	cd core && $(GO) fmt ./...

lint:
	@echo "Linting code..."
	cd core && golangci-lint run

## Uninstall
uninstall:
	@echo "Uninstalling RavenForge..."
	systemctl stop ravenforged || true
	systemctl disable ravenforged || true
	rm -f $(PREFIX)/bin/$(DAEMON)
	rm -f $(PREFIX)/bin/$(CLI)
	rm -f /etc/systemd/system/ravenforged.service
	systemctl daemon-reload
	@echo "Uninstalled. Config and data preserved in $(CONFIG_DIR) and $(DATA_DIR)"

## Help
help:
	@echo "RavenForge Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build targets:"
	@echo "  build          Build all binaries"
	@echo "  build-daemon   Build daemon only"
	@echo "  build-cli      Build CLI only"
	@echo ""
	@echo "Install targets:"
	@echo "  install        Full installation"
	@echo "  install-bin    Install binaries only"
	@echo "  install-config Install configuration"
	@echo "  install-service Install systemd service"
	@echo "  install-tools  Install tool manifests"
	@echo ""
	@echo "Docker targets:"
	@echo "  docker         Build all Docker images"
	@echo "  docker-push    Push images to registry"
	@echo ""
	@echo "Test targets:"
	@echo "  test           Run all tests"
	@echo "  test-short     Run short tests"
	@echo "  test-coverage  Run tests with coverage"
	@echo ""
	@echo "Other targets:"
	@echo "  clean          Clean build artifacts"
	@echo "  deps           Install Go dependencies"
	@echo "  fmt            Format code"
	@echo "  lint           Run linter"
	@echo "  uninstall      Uninstall RavenForge"
	@echo "  help           Show this help"
