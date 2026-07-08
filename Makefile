# cc-switch-server Makefile
# ===========================
#
# Targets:
#   make build          Build binary for current platform
#   make build-static   Build statically linked binary (CGO_ENABLED=0)
#   make build-linux    Cross-compile for Linux amd64 (static)
#   make build-all      Build for all supported platforms
#   make install        Install to /opt/cc-switch-server (requires sudo)
#   make uninstall      Remove installation
#   make docker         Build Docker image
#   make docker-run     Run Docker container locally
#   make clean          Remove build artifacts
#   make test           Run tests
#   make fmt            Format code

BINARY       := cc-switch
VERSION      := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
BUILD_TIME   := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Installation paths
PREFIX       := /opt/cc-switch-server
DATA_DIR     := /var/lib/cc-switch-server
SYSTEMD_DIR  := /etc/systemd/system

# Go build settings
GO           := go
GOFLAGS      := -trimpath
LDFLAGS      := -s -w \
                -X main.Version=$(VERSION) \
                -X main.BuildTime=$(BUILD_TIME) \
                -X main.GitCommit=$(GIT_COMMIT)

# Colors
GREEN  := \033[0;32m
YELLOW := \033[0;33m
RED    := \033[0;31m
NC     := \033[0m # No Color

.PHONY: all build build-static build-linux build-all \
        install uninstall reinstall \
        docker docker-run docker-push \
        clean test fmt check help

# Default target
all: build

help: ## Show this help
	@echo "cc-switch-server $(VERSION)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "$(GREEN)%-18s$(NC) %s\n", $$1, $$2}'

# ---- Build ----

build: ## Build binary for current platform
	@echo "$(YELLOW)Building $(BINARY) $(VERSION)...$(NC)"
	$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY) .
	@echo "$(GREEN)Built: $(BINARY) ($$(du -h $(BINARY) | cut -f1))$(NC)"

build-static: ## Build statically linked binary
	@echo "$(YELLOW)Building static $(BINARY) $(VERSION)...$(NC)"
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS) -extldflags '-static'" -o $(BINARY) .
	@echo "$(GREEN)Built: $(BINARY) ($$(du -h $(BINARY) | cut -f1))$(NC)"

build-linux: ## Cross-compile static binary for Linux amd64
	@echo "$(YELLOW)Cross-compiling for linux/amd64...$(NC)"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS) -extldflags '-static'" -o $(BINARY) .
	@echo "$(GREEN)Built: $(BINARY) ($(shell du -h $(BINARY) | cut -f1))$(NC)"

build-all: ## Build for all supported platforms
	@echo "$(YELLOW)Building for all platforms...$(NC)"
	@mkdir -p dist
	# Linux amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 .
	# Linux arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 .
	# macOS amd64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
		$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 .
	# macOS arm64 (Apple Silicon)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 .
	@echo "$(GREEN)Built in dist/:$(NC)"
	@ls -lh dist/

# ---- Install ----

install: build-static ## Install to $(PREFIX) and register systemd service
	@echo "$(YELLOW)Installing to $(PREFIX)...$(NC)"
	@if [ "$$(id -u)" != "0" ]; then \
		echo "$(RED)Error: install requires root. Use: sudo make install$(NC)"; \
		exit 1; \
	fi
	# Directories
	install -d $(PREFIX)
	install -d $(PREFIX)/web
	install -d $(DATA_DIR)/backups
	# Binary
	install -m 755 $(BINARY) $(PREFIX)/$(BINARY)
	# Web files
	install -m 644 web/* $(PREFIX)/web/
	# Systemd
	install -m 644 deploy/cc-switch-server.service $(SYSTEMD_DIR)/
	systemctl daemon-reload
	systemctl enable cc-switch-server
	@echo ""
	@echo "$(GREEN)Installation complete!$(NC)"
	@echo ""
	@echo "  Start service:  sudo systemctl start cc-switch-server"
	@echo "  Check status:   sudo systemctl status cc-switch-server"
	@echo "  View logs:      sudo journalctl -u cc-switch-server -f"
	@echo "  Web panel:      http://<server-ip>:9876"
	@echo ""

uninstall: ## Remove installation
	@echo "$(YELLOW)Uninstalling...$(NC)"
	@if [ "$$(id -u)" != "0" ]; then \
		echo "$(RED)Error: uninstall requires root. Use: sudo make uninstall$(NC)"; \
		exit 1; \
	fi
	-systemctl stop cc-switch-server 2>/dev/null
	-systemctl disable cc-switch-server 2>/dev/null
	rm -f $(SYSTEMD_DIR)/cc-switch-server.service
	systemctl daemon-reload
	rm -rf $(PREFIX)
	@echo "$(GREEN)Binary removed.$(NC)"
	@echo "$(YELLOW)Data at $(DATA_DIR) preserved. Remove manually if needed: rm -rf $(DATA_DIR)$(NC)"

reinstall: uninstall install ## Uninstall then reinstall

# ---- Docker ----

docker: ## Build Docker image
	@echo "$(YELLOW)Building Docker image...$(NC)"
	docker build -t cc-switch-server:$(VERSION) -t cc-switch-server:latest .
	@echo "$(GREEN)Docker image built: cc-switch-server:$(VERSION)$(NC)"

docker-run: ## Run Docker container locally (port 9876)
	@echo "$(YELLOW)Starting container...$(NC)"
	docker run -d --name cc-switch-server \
		-p 9876:9876 \
		-v cc-switch-data:/var/lib/cc-switch-server \
		cc-switch-server:latest
	@echo "$(GREEN)Container started. Web panel: http://localhost:9876$(NC)"

docker-stop: ## Stop and remove Docker container
	docker stop cc-switch-server 2>/dev/null || true
	docker rm cc-switch-server 2>/dev/null || true

docker-push: ## Push Docker image to registry
	@if [ -z "$(REGISTRY)" ]; then \
		echo "$(RED)Error: REGISTRY not set. Use: make docker-push REGISTRY=myregistry.com/$(NC)"; \
		exit 1; \
	fi
	docker tag cc-switch-server:$(VERSION) $(REGISTRY)/cc-switch-server:$(VERSION)
	docker tag cc-switch-server:$(VERSION) $(REGISTRY)/cc-switch-server:latest
	docker push $(REGISTRY)/cc-switch-server:$(VERSION)
	docker push $(REGISTRY)/cc-switch-server:latest
	@echo "$(GREEN)Pushed to $(REGISTRY)$(NC)"

# ---- Development ----

test: ## Run tests
	$(GO) test -v -race -cover ./...

fmt: ## Format code
	$(GO) fmt ./...
	$(GO) vet ./...

check: ## Run all checks (fmt + vet + test)
	@$(MAKE) fmt
	@$(MAKE) test

# ---- Cleanup ----

clean: ## Remove build artifacts
	@echo "$(YELLOW)Cleaning...$(NC)"
	rm -f $(BINARY)
	rm -rf dist/
	@echo "$(GREEN)Done.$(NC)"
