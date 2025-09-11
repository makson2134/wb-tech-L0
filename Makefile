.PHONY: up up-logs up-kafka up-silent down restart logs test test-v build clean
.PHONY: pprof-cpu pprof-mem pprof-goroutines pprof-web help status

include .env
export

up:
	@docker compose up -d
	@echo "Services started. Use 'make logs' to see logs"

up-logs:
	@docker compose up -d
	@docker compose logs -f server producer postgres

up-kafka:
	@docker compose up

up-silent:
	@docker compose up -d > /dev/null 2>&1

down:
	@docker compose down
	@echo "Services stopped."

restart: down up

logs:
	@docker compose logs -f

logs-%:
	@docker compose logs -f $*

test:
	@go test ./... 

test-v:
	@go test -v ./...

test-cover:
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out

status:
	@docker compose ps
