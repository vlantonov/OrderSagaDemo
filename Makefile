IMAGE_PREFIX ?= ghcr.io/vladiant/ordersagademo
IMAGE_TAG    ?= dev

.PHONY: generate build test vet lint up down \
        docker-build helm-lint helm-template ci vuln

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

# Build all three service images using repo-root context (R-5).
# Override IMAGE_TAG for versioned builds: make docker-build IMAGE_TAG=v0.1.1
docker-build:
	docker build -f services/order/Dockerfile     -t $(IMAGE_PREFIX)-order:$(IMAGE_TAG)     .
	docker build -f services/inventory/Dockerfile -t $(IMAGE_PREFIX)-inventory:$(IMAGE_TAG) .
	docker build -f services/payment/Dockerfile   -t $(IMAGE_PREFIX)-payment:$(IMAGE_TAG)   .

helm-lint:
	helm lint deploy/helm/ordersagademo

helm-template:
	helm template ordersagademo deploy/helm/ordersagademo

# Run the full local quality gate (matches CI build-test + lint jobs).
ci:
	go build ./...
	go vet ./...
	go test -race -count=1 ./...
	golangci-lint run

# Security scan — non-blocking by convention (D-3 accepted risk).
# govulncheck pinned to v1.1.4 (ADR-002): the last x/vuln release whose go.mod
# requires <= Go 1.22; @latest (v1.7.0) requires Go >= 1.25 and fails to install.
vuln:
	go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
	govulncheck ./...
