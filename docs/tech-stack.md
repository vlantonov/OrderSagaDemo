# Tech Stack — OrderSagaDemo

**Version:** 0.1.0  
**Date:** 2026-09-06  
**Owner:** System Architect  

---

## 1. Primary Language & Runtime

| Item | Choice | Rationale |
|---|---|---|
| Language | **Go 1.22** | First-class gRPC + Kafka + OTel SDK support; produces small static binaries ideal for distroless images; straightforward goroutine concurrency for saga handlers; fast builds; wide portfolio-reviewer familiarity. (Pinned at 1.22 — the installed runtime is Go 1.22.2; `go.mod` declares `go 1.22`.) |
| Module path | `github.com/vladiant/ordersagademo` | Matches the GitHub namespace; single Go module at repo root. |
| Module layout | Single `go.mod` at repo root | All three services share generated proto code in `internal/gen/`; reduces toolchain ceremony for a demo. |

---

## 2. Build System & Dependency Management

| Item | Choice | Version / Notes |
|---|---|---|
| Build orchestration | **GNU Make** | Single `Makefile` at repo root with targets: `build`, `test`, `lint`, `proto`, `docker-build`, `compose-up`, `kind-deploy`. |
| Go dependency manager | **Go modules** (`go mod`) | Built into Go 1.22; `go.sum` committed. |
| Proto toolchain | **buf** | `v1.35.x`; handles `buf lint`, `buf generate`, and breaking-change detection. |
| Proto plugins (buf-managed) | `protoc-gen-go` v1.34, `protoc-gen-go-grpc` v1.4 | Declared in `buf.gen.yaml`; no system-level `protoc` required. |
| Linter | **golangci-lint** | `v1.60.x`; config in `.golangci.yml` at repo root. Enabled linters: `errcheck`, `govet`, `staticcheck`, `goimports`, `revive`, `gosec`. |
| Security scan | **govulncheck** | `latest` via `go install golang.org/x/vuln/cmd/govulncheck@latest`; run in `make vuln`. |
| Formatter | `gofmt` / `goimports` | Enforced by golangci-lint; no separate step needed. |

---

## 3. gRPC

| Item | Choice | Version |
|---|---|---|
| gRPC library | `google.golang.org/grpc` | `v1.64.x` |
| Protobuf runtime | `google.golang.org/protobuf` | `v1.34.x` |
| OTel gRPC interceptors | `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` | `v0.54.0` |

The only `.proto` file is `proto/inventory/v1/inventory.proto`; generated Go stubs land in `internal/gen/inventory/v1/`.

---

## 4. Kafka Client

| Item | Choice | Version | Notes |
|---|---|---|---|
| Kafka client library | `github.com/segmentio/kafka-go` | `v0.4.x` | Pure Go, no CGo, no librdkafka; simplifies multi-stage Docker builds on Alpine/distroless. |
| Delivery semantics | At-least-once | — | Per assumption A-5 in SRS. |
| Serialisation | **JSON** | — | `encoding/json` from stdlib; no Schema Registry container needed. Event schemas are documented Go structs. |
| Consumer groups | Configurable via env var | — | `KAFKA_CONSUMER_GROUP` per service; never hardcoded (FR-9). |

---

## 5. OpenTelemetry SDK

All three signals (metrics, logs, traces) exported via OTLP/gRPC to the shared OTel Collector.

| Package | Version |
|---|---|
| `go.opentelemetry.io/otel` | `v1.29.0` |
| `go.opentelemetry.io/otel/sdk` | `v1.29.0` |
| `go.opentelemetry.io/otel/sdk/log` | `v0.5.0` |
| `go.opentelemetry.io/otel/sdk/metric` | `v1.29.0` |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | `v1.29.0` |
| `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc` | `v1.29.0` |
| `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc` | `v0.5.0` |
| `go.opentelemetry.io/contrib/propagators/b3` | `v1.29.0` (optional) |
| W3C TraceContext propagator | included in `go.opentelemetry.io/otel` | Used for gRPC and Kafka header injection |

---

## 6. Observability Stack — Image Tags

All images are pinned to a specific tag. Pull happens at `docker compose build` / `docker compose up --pull always`; no internet access required at container start-up (NFR-12).

| Component | Image | Tag |
|---|---|---|
| **OTel Collector** | `otel/opentelemetry-collector-contrib` | `0.104.0` |
| **Prometheus** | `prom/prometheus` | `v2.53.1` |
| **Loki** | `grafana/loki` | `3.1.1` |
| **Tempo** | `grafana/tempo` | `2.5.0` |
| **Grafana** | `grafana/grafana` | `11.1.4` |
| **Kafka** | `confluentinc/cp-kafka` | `7.7.0` (KRaft mode, no Zookeeper) |

---

## 7. Container Base Images

| Stage | Image | Notes |
|---|---|---|
| Build | `golang:1.22-alpine` | Go compiler + Alpine tools |
| Runtime | `gcr.io/distroless/static-debian12` | No shell, no package manager; ~2 MB base; statically linked Go binary drops straight in |

---

## 8. Local Orchestration

| Item | Choice | Version |
|---|---|---|
| Docker Compose | Docker Compose v2 (plugin) | `>=2.27` (ships with Docker Desktop 4.30 / Docker Engine 26) |
| Compose file version | `3.9` syntax | Compatible with Compose v2 |

---

## 9. Kubernetes & Helm

| Item | Choice | Version / Notes |
|---|---|---|
| Local cluster | **kind** | `v0.24.0` |
| Kubernetes API target | `1.30` | `apiVersion: apps/v1` (Deployments), `v1` (Services, ConfigMaps, Secrets) |
| Helm | `v3.16.x` | CLI only; chart committed to `deploy/helm/ordersagademo/` |
| Helm chart distribution | **Repo-only** | Chart lives in the repo; no external chart registry or OCI push required (OQ-10). |
| `kubeVersion` constraint in `Chart.yaml` | `>=1.28.0` | Allows kind 0.22+ and minikube users on recent clusters |

---

## 10. Test Frameworks & Scope

**Scope for this iteration (OQ-9): Unit tests + Integration tests against the running Compose stack. No contract tests (buf lint covers the proto contract).**

| Layer | Tool | Notes |
|---|---|---|
| Unit | Go built-in `testing` | One `_test.go` per internal package; no external runtime required. |
| Assertions | `github.com/stretchr/testify` `v1.9.x` | `require` and `assert` packages; avoids verbose `if` checks. |
| Integration | `github.com/testcontainers/testcontainers-go` `v0.31.x` | Spins up Kafka + the three service containers; exercises happy path and compensation path via HTTP + Kafka consumer assertions. |
| Make target | `make test-unit` / `make test-integration` | Integration tests guarded by `//go:build integration` tag; not run by default. |

---

## 11. Required Make Targets (QA Contract)

The QA Engineer must be able to run all of the following without any setup beyond `go`, `docker`, `make`, `buf`, and `golangci-lint` being on `PATH`:

| Target | Command | What it does |
|---|---|---|
| `proto` | `make proto` | `buf generate`; regenerates `internal/gen/` |
| `build` | `make build` | `go build ./cmd/...` for all three services |
| `test-unit` | `make test-unit` | `go test ./...` (unit only) |
| `lint` | `make lint` | `golangci-lint run ./...` |
| `vuln` | `make vuln` | `govulncheck ./...` |
| `docker-build` | `make docker-build` | Builds all three service images |
| `compose-up` | `make compose-up` | `docker compose -f deploy/docker-compose/docker-compose.yml up -d` |
| `compose-down` | `make compose-down` | `docker compose … down -v` |
| `test-integration` | `make test-integration` | Runs integration test suite (requires Docker) |
| `kind-deploy` | `make kind-deploy` | Creates kind cluster, loads images, applies manifests |
