# ─── ViewAura Makefile ────────────────────────────────────────────────────────
#
# Development workflow:
#   make dev            — swagger + wire + build in one shot
#   make docker-up      — start local stack (Postgres, Redis, Kafka, Temporal)
#   make migrate-up     — apply all pending Postgres migrations
#
# Code generation:
#   make swagger        — regenerate docs/swagger.json + docs/swagger.yaml
#   make wire           — regenerate Wire dependency injection graph (cmd/api)
#   make wire-worker    — regenerate Wire dependency injection graph (cmd/worker)
#
# Build:
#   make build          — compile API binary
#   make build-worker   — compile worker binary
#   make docker-build   — build both Docker images
#
# Quality:
#   make test           — run all tests with race detector
#   make test-coverage  — tests with HTML coverage report
#   make lint           — run golangci-lint
#
# Infrastructure:
#   make migrate-up     — apply migrations
#   make migrate-down   — roll back last migration
#   make kafka-topics   — create Kafka topics in local env
#   make infra-plan     — terraform plan
#   make infra-apply    — terraform apply
#   make deploy-staging — helm deploy to staging
#   make deploy-prod    — helm deploy to production
#
# Prerequisites:
#   go install github.com/swaggo/swag/cmd/swag@latest
#   go install github.com/google/wire/cmd/wire@latest
#   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
#   go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# ─── Configuration ────────────────────────────────────────────────────────────

BINARY       := bin/viewaura-api
WORKER_BIN   := bin/viewaura-worker
CMD_DIR      := ./cmd/api
WORKER_DIR   := ./cmd/worker
DOCS_DIR     := ./docs
API_IMAGE    := ghcr.io/viewaura/viewaura-api
WORKER_IMAGE := ghcr.io/viewaura/viewaura-worker
GIT_SHA      := $(shell git rev-parse --short HEAD)
VERSION      := $(shell git describe --tags --always --dirty)
DB_DSN       ?= postgres://viewaura:viewaura@localhost:5432/viewaura?sslmode=disable

SWAG_FLAGS := \
	--generalInfo   docs/doc.go \
	--dir           . \
	--output        $(DOCS_DIR) \
	--parseDependency \
	--parseInternal \
	--outputTypes   go,json,yaml \
	--instanceName  viewaura

.PHONY: all swagger swagger-serve wire wire-worker build build-worker docker-build \
        docker-push docker-up docker-down docker-logs test test-coverage test-integration \
        lint dev migrate-up migrate-down migrate-status kafka-topics \
        infra-init infra-plan infra-apply deploy-staging deploy-prod rollback \
        tools clean help app run

all: dev

# ─── Code generation ──────────────────────────────────────────────────────────

## swagger: Regenerate OpenAPI docs from swag annotations.
##          Output: docs/docs.go  docs/swagger.json  docs/swagger.yaml
swagger:
	swag init --generalInfo cmd/api/main.go --dir internal/modules/auth/api/http,internal/modules/movie/api/http,internal/modules/rating/api/http,internal/modules/review/api/http,internal/modules/watchlist/api/http,internal/modules/social/api/http,internal/modules/notification/api/http,internal/modules/payment/api/http,internal/modules/upload/api/http,internal/modules/quota/api/http,internal/modules/moderation/api/http,internal/modules/user/api,internal/modules/profile/api/http --output internal/docs --outputTypes go,json,yaml --parseDependency --parseInternal --parseDepth 5

## swagger-serve: Run the API in local mode and open the Swagger UI.
swagger-serve: swagger build
	@echo "→ starting server (Swagger UI at http://localhost:8080/swagger/index.html)"
	APP_ENV=local $(BINARY)

## Alias
.PHONY: docs
docs: swagger

## wire: Regenerate Wire dependency injection graph for cmd/api.
wire:
	@echo "→ running wire (cmd/api)..."
	cd $(CMD_DIR) && wire
	@echo "✓ wire_gen.go updated"

## wire-worker: Regenerate Wire dependency injection graph for cmd/worker.
wire-worker:
	@echo "→ running wire (cmd/worker)..."
	cd $(WORKER_DIR) && wire
	@echo "✓ wire_gen.go updated"

# ─── Build ────────────────────────────────────────────────────────────────────

## build: Compile the API binary.
build:
	@echo "→ building $(BINARY)..."
	CGO_ENABLED=1 go build \
		-ldflags="-s -w -X main.Version=$(VERSION)" \
		-o $(BINARY) $(CMD_DIR)
	@echo "✓ $(BINARY) ready"

## build-worker: Compile the worker binary.
build-worker:
	@echo "→ building $(WORKER_BIN)..."
	CGO_ENABLED=1 go build \
		-ldflags="-s -w -X main.Version=$(VERSION)" \
		-o $(WORKER_BIN) $(WORKER_DIR)
	@echo "✓ $(WORKER_BIN) ready"

## docker-build: Build API and worker Docker images.
docker-build:
	docker build -f deployments/docker/Dockerfile.api \
		-t $(API_IMAGE):$(GIT_SHA) \
		-t $(API_IMAGE):latest .
	docker build -f deployments/docker/Dockerfile.worker \
		-t $(WORKER_IMAGE):$(GIT_SHA) \
		-t $(WORKER_IMAGE):latest .

## docker-push: Push images to the container registry.
docker-push:
	docker push $(API_IMAGE):$(GIT_SHA)
	docker push $(API_IMAGE):latest
	docker push $(WORKER_IMAGE):$(GIT_SHA)
	docker push $(WORKER_IMAGE):latest

# ─── Local development stack ──────────────────────────────────────────────────

## docker-up: Start the local dev stack (Postgres, Redis, Kafka, Temporal, ClickHouse).
docker-base_up:
	@echo "→ starting local dev environment..."
	docker compose -f deployments/docker/docker-compose.yml up -d
	@echo ""
	@echo "  API:        http://localhost:8080"
	@echo "  Swagger UI: http://localhost:8080/swagger/index.html"
	@echo "  Temporal:   http://localhost:8088"
	@echo "  ClickHouse: http://localhost:8123"
	@echo ""

docker-up:
	@echo "→ starting FULL stack (base + worker)..."
	docker compose -f deployments/docker/docker-compose.yml --profile worker up -d

	@echo ""
	@echo "  API:        http://localhost:8080"
	@echo "  Swagger UI: http://localhost:8080/swagger/index.html"
	@echo "  Temporal:   http://localhost:8088"
	@echo "  ClickHouse: http://localhost:8123"
	@echo ""

docker-worker_up:
	@echo "→ starting worker..."
	docker compose -f deployments/docker/docker-compose.yml --profile worker up -d worker

## docker-down: Stop and remove the local dev stack.
docker-down:
	docker compose -f deployments/docker/docker-compose.yml down -v

## docker-logs: Follow logs from API and worker containers.
docker-logs:
	docker compose -f deployments/docker/docker-compose.yml logs -f api worker

# ─── Quality ──────────────────────────────────────────────────────────────────

## test: Run all tests with race detector.
test:
	go test -race ./...

## test-coverage: Run tests and generate an HTML coverage report.
test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ coverage report: coverage.html"

## test-integration: Run integration tests (requires docker-up).
test-integration:
	go test -v -tags=integration -timeout=120s ./...

## lint: Run golangci-lint.
lint:
	golangci-lint run ./...

# ─── Dev shortcut ─────────────────────────────────────────────────────────────

## dev: Full regeneration cycle — swagger, wire (api + worker), build both.
dev: swagger wire wire-worker build build-worker

# ─── Database migrations ──────────────────────────────────────────────────────

## migrate-up: Apply all pending Postgres migrations.
migrate-up:
	@echo "→ running migrations..."
	migrate -path infrastructure/postgres/migrations \
	        -database "$(DB_DSN)" up
	@echo "✓ migrations applied"

## migrate-down: Roll back migrations -ll, 1, 2, etc default 1.
migrate-down:
	@if [ "$(filter -all,$(MAKECMDGOALS))" ]; then \
		migrate -path infrastructure/postgres/migrations \
			-database "$(DB_DSN)" down -all; \
	elif [ -n "$(word 2,$(MAKECMDGOALS))" ]; then \
		migrate -path infrastructure/postgres/migrations \
			-database "$(DB_DSN)" down $(word 2,$(MAKECMDGOALS)); \
	else \
		migrate -path infrastructure/postgres/migrations \
			-database "$(DB_DSN)" down 1; \
	fi

# Prevent make from treating extra args as targets
%:
	@:

## migrate-status: Show current migration version.
migrate-status:
	migrate -path infrastructure/postgres/migrations \
	        -database "$(DB_DSN)" version

# ─── Kafka ────────────────────────────────────────────────────────────────────

## kafka-topics: Create all Kafka topics in the local dev environment.
##               Topics are also created automatically by the kafka-init
##               container on docker-up — use this for manual reset only.
kafka-topics:
	@echo "→ creating Kafka topics..."
	@for topic in \
		user.events movie.events ratings rating.aggregates reviews \
		watch_events notifications search_index moderation moderation.decided \
		uploads.completed uploads.failed payment.events social.events \
		workflow_events box_office; do \
		docker exec viewaura-kafka-1 \
			kafka-topics --bootstrap-server localhost:9092 \
			--create --if-not-exists \
			--topic $$topic \
			--partitions 4 \
			--replication-factor 1; \
	done
	@echo "✓ topics ready"

# ─── Infrastructure (Terraform + Helm) ───────────────────────────────────────

## infra-init: Initialise Terraform providers and backend.
infra-init:
	cd deployments/terraform && terraform init

## infra-plan: Preview infrastructure changes (production).
infra-plan:
	cd deployments/terraform && \
		terraform plan -var-file=environments/production.tfvars

## infra-apply: Apply infrastructure changes (production).
infra-apply:
	cd deployments/terraform && \
		terraform apply -var-file=environments/production.tfvars

## deploy-staging: Helm deploy to staging cluster.
deploy-staging:
	helm upgrade --install viewaura deployments/helm/viewaura \
		--namespace viewaura \
		--create-namespace \
		--set global.imageTag=$(GIT_SHA) \
		--set global.env=staging \
		--values deployments/helm/viewaura/values.yaml \
		--values deployments/helm/viewaura/values.staging.yaml \
		--wait --timeout=5m

## deploy-prod: Helm deploy to production cluster.
deploy-prod:
	helm upgrade --install viewaura deployments/helm/viewaura \
		--namespace viewaura \
		--create-namespace \
		--set global.imageTag=$(GIT_SHA) \
		--set global.env=production \
		--values deployments/helm/viewaura/values.yaml \
		--wait --timeout=10m

## rollback: Roll back the last Helm release.
rollback:
	helm rollback viewaura --namespace viewaura

# ─── Tool installation ────────────────────────────────────────────────────────

## tools: Install all required CLI tools.
tools:
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/google/wire/cmd/wire@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# ─── Clean ────────────────────────────────────────────────────────────────────

## clean: Remove compiled artefacts and generated docs.
clean:
	rm -rf bin/ \
		$(DOCS_DIR)/docs.go \
		$(DOCS_DIR)/swagger.json \
		$(DOCS_DIR)/swagger.yaml \
		coverage.out \
		coverage.html

# ─── Help ─────────────────────────────────────────────────────────────────────

## help: Print available targets.
help:
	@grep -E '^## [a-zA-Z_-]+:' $(MAKEFILE_LIST) \
		| sed 's/## //' \
		| column -t -s ':'

## app: Start full app (infra + migrate + swagger + run API)
app: docker-base_up
	@echo "→ waiting for postgres..."
	sleep 5
	$(MAKE) migrate-up
	$(MAKE) swagger
	$(MAKE) build
	@echo "→ starting API..."
	APP_ENV=local $(BINARY)

run: swagger build
	APP_ENV=local $(BINARY)