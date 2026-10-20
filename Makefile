GOLANGCI_VERSION := v2.14.0
REDOCLY_VERSION := 2.57.0
COMPOSE := docker compose -f deploy/docker-compose.yml

.PHONY: build test test-unit lint lint-api gen gen-check up down logs migrate seed create-admin

build:
	go build ./...

# Unit and integration tests. Integration tests start Postgres with testcontainers (needs Docker).
test:
	go test -race ./...

# Unit tests only; integration tests skip themselves under -short.
test-unit:
	go test -race -short ./...

# Uses a local golangci-lint if it matches, otherwise the pinned Docker image.
lint:
	@if golangci-lint version 2>/dev/null | grep -q "$(GOLANGCI_VERSION:v%=%)"; then \
		golangci-lint run ./...; \
	else \
		docker run --rm -v "$(CURDIR)":/src -w /src golangci/golangci-lint:$(GOLANGCI_VERSION) golangci-lint run ./...; \
	fi

# Lints the API contract api/openapi.yaml (CI runs the same image).
lint-api:
	docker run --rm -v "$(CURDIR)/api":/spec -w /spec redocly/cli:$(REDOCLY_VERSION) lint openapi.yaml

# Regenerate sqlc code after editing migrations/ or internal/store/queries/.
gen:
	go tool sqlc generate

# Fails if generated code is out of date (used by CI).
gen-check:
	go tool sqlc diff

up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f api agent

migrate:
	$(COMPOSE) run --rm pcctl migrate up

seed:
	$(COMPOSE) run --rm pcctl seed

# make create-admin EMAIL=admin@example.com NAME="Quan tri"  (prompts for the password)
create-admin:
	$(COMPOSE) run --rm pcctl create-admin -email "$(EMAIL)" -name "$(NAME)"
