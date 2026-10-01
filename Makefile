.PHONY: help up down nuke logs ps fix fmt lint vet test check coverage coverage-html

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

## --- Test databases ---

up: ## Start the test databases and wait until they are healthy
	docker compose up -d --wait

down: ## Stop the test databases
	docker compose down

nuke: ## Stop the test databases and destroy their volumes
	docker compose down -v

logs: ## Follow database logs
	docker compose logs -f

ps: ## Show the test databases and their health
	docker compose ps

## --- Quality ---

fix: ## Apply go fix modernizations
	go fix ./...

fmt: ## Format code
	go fmt ./...

lint: ## Run golangci-lint
	golangci-lint run

vet: ## Run go vet
	go vet ./...

check: fix fmt vet lint ## Everything CI checks, locally

## --- Tests ---

test: ## Run tests
	go test ./...

coverage: ## Coverage with a per-function summary
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

coverage-html: coverage ## Open the coverage report in a browser
	go tool cover -html=coverage.out
