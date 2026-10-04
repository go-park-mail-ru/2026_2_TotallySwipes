.PHONY: run up up-all migrate-up migrate-down migrate-create test

run:
	set -a && . ./local.env && set +a && go run ./cmd/api

up:
	set -a && . ./docker.env && set +a && docker compose up postgres redis -d

up-all:
	set -a && . ./docker.env && set +a && docker compose up --build

migrate-up:
	set -a && . ./docker.env && set +a && docker compose --profile tools run --rm migrate -path=/migrations -database "$$DATABASE_URL" up

migrate-down:
	set -a && . ./docker.env && set +a && docker compose --profile tools run --rm migrate -path=/migrations -database "$$DATABASE_URL" down 1

migrate-create:
	docker run --rm -v $(CURDIR)/db/migrations:/migrations migrate/migrate create -ext sql -dir /migrations -seq $(name)

test:
	go test ./...

.PHONY: local-up local-seed local-down
local-up:
	docker compose -f docker-compose.local.yml up -d --wait

local-seed:
	python3 db/seeds/prepare_demo_photos.py
	docker compose -f docker-compose.local.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d dating_app < db/seeds/demo_profiles.sql

local-down:
	docker compose -f docker-compose.local.yml down
