# SubTrends Makefile — local development, testing, and Docker helpers

SHELL := /bin/zsh

# Load environment variables from .env if present (non-fatal if missing)
-include .env
export $(shell sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/\1/p' .env 2>/dev/null)

# Python variables
PYTHON          ?= python3
VENV_DIR        ?= .venv
PIP             ?= $(VENV_DIR)/bin/pip
PYTHON_VENV     ?= $(VENV_DIR)/bin/python

# Docker variables
DOCKER_IMAGE    ?= subtrends
CONTAINER_NAME  ?= subtrends-bot
ENV_FILE        ?= .env

.PHONY: help venv install run test lint fmt clean docker-build docker-run docker-stop ensure-data-dir init-env init

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

ensure-data-dir: ## Create local data directory if missing
	@mkdir -p data

venv: ## Create virtual environment
	@if [ ! -d $(VENV_DIR) ]; then $(PYTHON) -m venv $(VENV_DIR); fi

install: venv ## Install dependencies into virtual environment
	$(PIP) install --upgrade pip
	$(PIP) install -r requirements.txt

run: ensure-data-dir ## Run the bot locally (uses variables from .env if present)
	@if [ -d $(VENV_DIR) ]; then \
		$(PYTHON_VENV) main.py; \
	else \
		$(PYTHON) main.py; \
	fi

test: ## Run tests with pytest
	@if [ -d $(VENV_DIR) ]; then \
		$(VENV_DIR)/bin/pytest -v; \
	else \
		pytest -v; \
	fi

lint: ## Run linting with ruff
	@if [ -d $(VENV_DIR) ]; then \
		$(VENV_DIR)/bin/ruff check .; \
	else \
		ruff check .; \
	fi

fmt: ## Format code with ruff
	@if [ -d $(VENV_DIR) ]; then \
		$(VENV_DIR)/bin/ruff format .; \
	else \
		ruff format .; \
	fi

clean: ## Clean virtual environment and cache files
	rm -rf $(VENV_DIR) __pycache__ .pytest_cache .ruff_cache

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
	  "# You can override the default values from config.py" \
	  "REDDIT_POST_LIMIT=7" \
	  "REDDIT_COMMENT_LIMIT=7" \
	  "REDDIT_TIMEFRAME=day" \
	  > .env
	@echo "Wrote .env from template. Fill in required values before running."

init: install ## One-shot: create venv and install dependencies

