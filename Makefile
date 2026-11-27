.PHONY: run up up-all migrate-up migrate-down migrate-create test seed

run:
	set -a && . ./.env && set +a && go run ./cmd/api

up:
	set -a && . ./.env && set +a && docker compose up postgres redis -d

up-all:
	set -a && . ./.env && set +a && docker compose up --build

migrate-up:
	set -a && . ./.env && set +a && docker compose --profile tools run --rm migrate -path=/migrations -database "$$DATABASE_URL" up

migrate-down:
	set -a && . ./.env && set +a && docker compose --profile tools run --rm migrate -path=/migrations -database "$$DATABASE_URL" down 1

migrate-create:
	docker run --rm -v $(CURDIR)/db/migrations:/migrations migrate/migrate create -ext sql -dir /migrations -seq $(name)

test:
	go test ./...

API ?= http://localhost:8080

seed:
	python3 scripts/seed/seed_demo.py --api $(API)

.PHONY: local-up local-down
local-up:
	docker compose -f docker-compose.local.yml up -d --wait

local-down:
	docker compose -f docker-compose.local.yml down

.PHONY: test-integration
test-integration:
	bash scripts/integration/run.sh


PROTO_FILES := $(shell find api/proto -type f -name '*.proto' | sort)
.PHONY: proto
proto:
	protoc \
		--proto_path=api/proto \
		--go_out=. \
		--go_opt=module=dating-app \
		--go-grpc_out=. \
		--go-grpc_opt=module=dating-app \
		$(PROTO_FILES)
.PHONY: run-auth run-profile run-gateway
run-auth:
	set -a && . ./deploy/env/auth.env && set +a && go run ./services/auth/cmd

run-profile:
	set -a && . ./deploy/env/profile.env && set +a && go run ./services/profile/cmd

run-gateway:
	set -a && . ./deploy/env/gateway.env && set +a && go run ./gateway/cmd
