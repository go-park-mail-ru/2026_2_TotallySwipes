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
