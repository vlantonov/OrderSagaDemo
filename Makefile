.PHONY: generate build test vet lint up down

generate:
	buf generate

build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run

up:
	docker compose -f deploy/docker-compose/docker-compose.yml up -d --build

down:
	docker compose -f deploy/docker-compose/docker-compose.yml down -v
