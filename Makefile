# ─── ViewAura Makefile ────────────────────────────────────────────────────────
#
# Usage:
#   make swagger        — regenerate docs/swagger.json + docs/swagger.yaml
#   make swagger-serve  — start the API locally and open the Swagger UI
#   make wire           — regenerate Wire dependency graph
#   make build          — compile the API binary
#   make test           — run all tests
#   make lint           — run golangci-lint
#   make dev            — swagger + wire + build in one shot
#
# Prerequisites:
#   go install github.com/swaggo/swag/cmd/swag@latest
#   go install github.com/google/wire/cmd/wire@latest
#   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

BINARY     := bin/viewaura-api
CMD_DIR    := ./cmd/api
DOCS_DIR   := ./docs
SWAG_FLAGS := \
	--generalInfo   docs/doc.go \
	--dir           . \
	--output        $(DOCS_DIR) \
	--parseDependency \
	--parseInternal \
	--outputTypes   go,json,yaml \
	--instanceName  viewaura

.PHONY: all swagger swagger-serve wire build test lint dev clean

all: dev

## swagger: Regenerate OpenAPI docs from swag annotations.
##          Output: docs/docs.go  docs/swagger.json  docs/swagger.yaml
swagger:
	@echo "→ generating swagger docs..."
	swag init $(SWAG_FLAGS)
	@echo "✓ docs written to $(DOCS_DIR)/"

## swagger-serve: Run the API in local mode and print the Swagger UI URL.
swagger-serve: swagger build
	@echo "→ starting server (Swagger UI at http://localhost:8080/swagger/index.html)"
	APP_ENV=local $(BINARY)

## wire: Regenerate the Wire dependency injection graph.
wire:
	@echo "→ running wire..."
	cd $(CMD_DIR) && wire
	@echo "✓ wire_gen.go updated"

## build: Compile the API binary.
build:
	@echo "→ building $(BINARY)..."
	go build -o $(BINARY) $(CMD_DIR)
	@echo "✓ $(BINARY) ready"

## test: Run all tests with race detector.
test:
	go test -race ./...

## lint: Run golangci-lint.
lint:
	golangci-lint run ./...

## dev: Full regeneration cycle — swagger, wire, then build.
dev: swagger wire build

## clean: Remove compiled artefacts.
clean:
	rm -rf bin/ $(DOCS_DIR)/docs.go $(DOCS_DIR)/swagger.json $(DOCS_DIR)/swagger.yaml