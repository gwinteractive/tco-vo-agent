export PROJECT_NAME=$(shell basename $(CURDIR)-v2)
export PROJECT_PATH=$(CURDIR)
export GOOGLE_PROJECT=$(shell gcloud config get-value core/project)
export OPENAI_KEY_NAME=FINYA_FRAUD_AGENT_OPENAI_KEY

CLOUD_FUNC_DIR := src/cloudfunction
FUNCTION_NAME ?= ProcessUsers
REGION ?= europe-west1
PROJECT_ID ?= $(shell gcloud config get-value project 2>/dev/null)
RUNTIME ?= go124

REQUIRES_OPENAI_API_KEY := build test tidy lint run local url logs describe delete update-env
ifneq (,$(filter $(MAKECMDGOALS),$(REQUIRES_OPENAI_API_KEY)))
ifndef OPENAI_API_KEY
$(error OPENAI_API_KEY is not set. Set it with: export OPENAI_API_KEY=your-key-here)
endif
endif

.PHONY: secrets help build test clean tidy lint run local url logs describe delete update-env update-finya-url

secrets: ## Create secrets
	./build/secrets.sh "$(OPENAI_KEY)"

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the cloud function locally
	@echo "Building function..."
	cd $(CLOUD_FUNC_DIR) && go build -o bin/$(FUNCTION_NAME) .

test: ## Run cloud function tests
	@echo "Running tests..."
	cd $(CLOUD_FUNC_DIR) && go test -v ./...

tidy: ## Run go mod tidy for the cloud function
	@echo "Tidying go.mod..."
	cd $(CLOUD_FUNC_DIR) && go mod tidy

lint: ## Run linter for the cloud function
	@echo "Running linter..."
	cd $(CLOUD_FUNC_DIR) && golangci-lint run || echo "golangci-lint not installed, skipping..."

clean: ## Clean cloud function build artifacts
	@echo "Cleaning..."
	cd $(CLOUD_FUNC_DIR) && rm -rf bin/ && go clean

run: ## Run the cloud function locally
	@echo "Running function locally on port 8080..."
	@echo "Set PORT environment variable to change the port"
	cd $(CLOUD_FUNC_DIR) && PORT=8080 go run .

local: run ## Alias for run

url: ## Get the deployed function URL
	@gcloud functions describe $(FUNCTION_NAME) \
		--gen2 \
		--region=$(REGION) \
		--format='value(serviceConfig.uri)' \
		--project=$(PROJECT_ID) 2>/dev/null || \
		echo "Error: Could not get function URL. Make sure the function is deployed."

logs: ## View deployed function logs
	@gcloud functions logs read $(FUNCTION_NAME) \
		--gen2 \
		--region=$(REGION) \
		--limit=50 \
		--project=$(PROJECT_ID)

describe: ## Describe the deployed function
	@gcloud functions describe $(FUNCTION_NAME) \
		--gen2 \
		--region=$(REGION) \
		--project=$(PROJECT_ID)

delete: ## Delete the deployed function
	@echo "Are you sure you want to delete $(FUNCTION_NAME)? [y/N]"
	@read -r confirm && [ "$$confirm" = "y" ] || exit 1
	@gcloud functions delete $(FUNCTION_NAME) \
		--gen2 \
		--region=$(REGION) \
		--project=$(PROJECT_ID)

update-finya-url: ## Update only FINYA_API_URL without touching other env vars
	@if [ -z "$(PROJECT_ID)" ]; then \
		echo "Error: PROJECT_ID is not set"; \
		exit 1; \
	fi
	@if [ -z "$(FINYA_API_URL)" ]; then \
		echo "Error: FINYA_API_URL is not set"; \
		exit 1; \
	fi
	@gcloud functions deploy $(FUNCTION_NAME) \
		--gen2 \
		--runtime=$(RUNTIME) \
		--region=$(REGION) \
		--source=$(CLOUD_FUNC_DIR) \
		--entry-point=$(FUNCTION_NAME) \
		--trigger-http \
		--update-env-vars="FINYA_API_URL=$(FINYA_API_URL)" \
		--project=$(PROJECT_ID)
