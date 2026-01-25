.PHONY: setup run test lint format clean docker-build docker-run help

# Python and venv configuration
PYTHON := python3
VENV := .venv
BIN := $(VENV)/bin

# Docker configuration
IMAGE_NAME := subtrends
IMAGE_TAG := latest

help: ## Show this help message
	@echo "SubTrends - Discord Bot for Reddit News Summaries"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

setup: ## Create virtual environment and install dependencies
	@echo "Creating virtual environment..."
	$(PYTHON) -m venv $(VENV)
	@echo "Upgrading pip..."
	$(BIN)/pip install --upgrade pip
	@echo "Installing dependencies..."
	$(BIN)/pip install -e ".[dev]"
	@echo ""
	@echo "Setup complete! Activate with: source $(VENV)/bin/activate"

run: ## Run the bot locally
	@if [ ! -f .env ]; then \
		echo "Error: .env file not found. Copy .env.example to .env and fill in values."; \
		exit 1; \
	fi
	$(BIN)/python -m src.main

test: ## Run tests
	$(BIN)/pytest tests/ -v

test-cov: ## Run tests with coverage
	$(BIN)/pytest tests/ -v --cov=src --cov-report=term-missing

lint: ## Run linting checks
	$(BIN)/ruff check src/ tests/
	$(BIN)/ruff format --check src/ tests/
	$(BIN)/mypy src/

format: ## Format code with ruff
	$(BIN)/ruff format src/ tests/
	$(BIN)/ruff check --fix src/ tests/

clean: ## Remove virtual environment and cache files
	rm -rf $(VENV)
	rm -rf __pycache__ .pytest_cache .mypy_cache .ruff_cache
	rm -rf .coverage htmlcov
	find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	find . -type f -name "*.pyc" -delete 2>/dev/null || true
	@echo "Cleaned up!"

docker-build: ## Build Docker image
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

docker-run: ## Run bot in Docker container
	@if [ ! -f .env ]; then \
		echo "Error: .env file not found. Copy .env.example to .env and fill in values."; \
		exit 1; \
	fi
	docker run --rm --env-file .env $(IMAGE_NAME):$(IMAGE_TAG)

docker-shell: ## Open a shell in the Docker container
	docker run --rm -it --env-file .env $(IMAGE_NAME):$(IMAGE_TAG) /bin/bash
