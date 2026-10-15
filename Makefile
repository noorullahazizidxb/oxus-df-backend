# File: Makefile
# Usage: Shortcut commands (make test, make up, ...). Run `make help` to list them.

SHELL := /bin/bash

DATABASE_URL ?= postgres://oxus:oxus@localhost:5432/oxus?sslmode=disable
COMPOSE      := docker compose -f deploy/compose/docker-compose.dev.yml
RUN_WITH_ENV := set -a && . ./.env && set +a &&

.PHONY: help tools generate oapi sqlc bootstrap migrate-up migrate-down migrate-new river-migrate \
	run-api run-worker build test test-integration lint fmt vuln up down logs

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

tools: ## Install code generators and linters
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install mvdan.cc/gofumpt@latest

oapi: ## Generate models + Gin server interface from api/v1/openapi.yaml
	oapi-codegen -config api/v1/oapi-codegen.yaml api/v1/openapi.yaml

sqlc: ## Generate type-safe Go from db/queries
	sqlc generate -f db/sqlc.yaml

generate: oapi sqlc ## Run every generator

bootstrap: tools generate ## First-time setup: tools, generators, go.sum
	go mod tidy

migrate-up: ## Apply SQL migrations
	migrate -path db/migrations -database "$(DATABASE_URL)" up

migrate-down: ## Roll back one SQL migration
	migrate -path db/migrations -database "$(DATABASE_URL)" down 1

migrate-new: ## Create a migration: make migrate-new name=add_customers
	migrate create -ext sql -dir db/migrations -seq $(name)

river-migrate: ## Apply River (job queue) migrations
	$(RUN_WITH_ENV) go run ./cmd/worker migrate

run-api: ## Run the HTTP API
	$(RUN_WITH_ENV) go run ./cmd/api

run-worker: ## Run the background worker
	$(RUN_WITH_ENV) go run ./cmd/worker

build: ## Build binaries into ./bin
	CGO_ENABLED=0 go build -trimpath -o bin/api ./cmd/api
	CGO_ENABLED=0 go build -trimpath -o bin/worker ./cmd/worker

test: ## Unit tests
	go test -race -coverprofile=coverage.out ./...

test-integration: ## Integration tests against real PostgreSQL (needs Docker)
	go test -race -tags=integration ./...

lint: ## golangci-lint (includes gosec)
	golangci-lint run ./...

fmt: ## gofumpt
	gofumpt -l -w .

vuln: ## Official Go vulnerability check
	govulncheck ./...

up: ## Start the full dev stack in Docker
	$(COMPOSE) up --build -d

down: ## Stop the dev stack
	$(COMPOSE) down

logs: ## Follow API and worker logs
	$(COMPOSE) logs -f api worker
