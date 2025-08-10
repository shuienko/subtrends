# SubTrends Makefile — local development, testing, and Docker helpers

SHELL := /bin/zsh

# Load environment variables from .env if present (non-fatal if missing)
-include .env
export $(shell sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/\1/p' .env 2>/dev/null)

# Go / project variables
GO              ?= go
PKG             ?= ./...
BIN_DIR         ?= bin
BINARY_NAME     ?= subtrends-bot
OUTPUT          ?= $(BIN_DIR)/$(BINARY_NAME)

# Docker variables
DOCKER_IMAGE    ?= subtrends
CONTAINER_NAME  ?= subtrends-bot
ENV_FILE        ?= .env

.PHONY: help build run test coverage fmt vet tidy lint check clean docker-build docker-run docker-stop ensure-data-dir init-env init

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

ensure-data-dir: ## Create local data directory if missing
	@mkdir -p data

build: ensure-data-dir ## Build the Go binary into $(OUTPUT)
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(OUTPUT) .

run: ensure-data-dir ## Run the bot locally (uses variables from .env if present)
	$(GO) run .

test: ## Run tests with race detector and coverage
	$(GO) test $(PKG) -race -covermode=atomic -coverprofile=coverage.out -count=1 -v

coverage: test ## Generate HTML coverage report at coverage.html
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

fmt: ## Format code using go fmt
	$(GO) fmt $(PKG)

vet: ## Run go vet static analysis
	$(GO) vet $(PKG)

tidy: ## Sync go.mod and go.sum
	$(GO) mod tidy

lint: vet ## Basic lint alias (currently runs go vet)
	@echo "Lint completed"

check: fmt vet test ## Run formatting, vet, and tests

clean: ## Clean build outputs and coverage
	rm -rf $(BIN_DIR) coverage.out coverage.html
	$(GO) clean -modcache

docker-build: ## Build Docker image $(DOCKER_IMAGE)
	docker build -t $(DOCKER_IMAGE) .

docker-run: ensure-data-dir ## Run Docker container with env from $(ENV_FILE) and mount data volume
	@if [ ! -f $(ENV_FILE) ]; then echo "$(ENV_FILE) not found. Create it or run 'make init-env' first."; exit 1; fi
	docker run --rm -it \
	  --name $(CONTAINER_NAME) \
	  --env-file $(ENV_FILE) \
	  -v $(PWD)/data:/app/data \
	  $(DOCKER_IMAGE)

docker-stop: ## Stop and remove running container
	-@docker rm -f $(CONTAINER_NAME) >/dev/null 2>&1 || true

init-env: ## Create a starter .env from README example (if missing)
	@if [ -f .env ]; then echo ".env already exists"; exit 0; fi
	@printf "%s\n" \
	  "# .env file" \
	  "# Get these from your Discord Developer Portal application" \
	  "DISCORD_BOT_TOKEN=" \
	  "" \
	  "# Get these from your Reddit App preferences (https://www.reddit.com/prefs/apps)" \
	  "REDDIT_CLIENT_ID=" \
	  "REDDIT_CLIENT_SECRET=" \
	  "" \
	  "# Get this from your OpenAI account dashboard" \
	  "OPENAI_API_KEY=" \
	  "" \
	  "# --- Optional Settings ---" \
	  "# You can override the default values from config.go" \
	  "REDDIT_POST_LIMIT=7" \
	  "REDDIT_COMMENT_LIMIT=7" \
	  "REDDIT_TIMEFRAME=day" \
	  > .env
	@echo "Wrote .env from template. Fill in required values before running."

init: tidy build ## One-shot: tidy modules and build


