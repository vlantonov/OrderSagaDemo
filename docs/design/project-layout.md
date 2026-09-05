# Project Layout — OrderSagaDemo

**Version:** 0.1.0  
**Date:** 2026-09-06  
**Status:** Ready for Developer  
**Purpose:** Authoritative directory tree that the Developer must implement into. Every path listed here is either a file to create or a directory to create. Paths marked `[generated]` are outputs of tooling (never hand-edit). Paths marked `[committed]` must be committed to version control.

---

## Repository Root

```
OrderSagaDemo/                        ← repo root / Docker build context for all services
├── LICENSE                           [committed] existing
├── README.md                         [committed] to be written by Developer
├── Makefile                          [committed] build orchestration (see tech-stack.md §11)
├── .golangci.yml                     [committed] golangci-lint configuration
├── go.mod                            [committed] module github.com/vladiant/ordersagademo
├── go.sum                            [committed]
│
├── cmd/                              one main package per binary
│   ├── order/
│   │   └── main.go                   [committed] wires config → telemetry → store → saga → HTTP server
│   ├── payment/
│   │   └── main.go                   [committed] wires config → telemetry → ledger → kafka consumer/producer
│   └── inventory/
│       └── main.go                   [committed] wires config → telemetry → stock store → gRPC server
│
├── internal/                         all non-generated application code; not importable outside module
│   │
│   ├── gen/                          [generated] proto code-gen output — do not hand-edit
│   │   └── inventory/v1/
│   │       ├── inventory.pb.go       [generated] by buf generate
│   │       └── inventory_grpc.pb.go  [generated] by buf generate
│   │
│   ├── telemetry/                    shared OTel bootstrap; imported by all three cmd/*/main.go
│   │   └── telemetry.go              TraceProvider + MeterProvider + LoggerProvider init; shutdown func
│   │
│   ├── messaging/                    shared Kafka helpers; imported by order, payment, inventory packages
│   │   ├── envelope.go               Event envelope struct + JSON marshal/unmarshal
│   │   └── propagation.go            W3C traceparent inject into / extract from kafka-go message Headers
│   │
│   ├── order/
│   │   ├── api/
│   │   │   └── handler.go            HTTP handler: POST /orders → calls saga.Orchestrator
│   │   ├── saga/
│   │   │   ├── orchestrator.go       State machine: state transitions + compensation logic
│   │   │   ├── orchestrator_test.go  Unit tests; uses fake store + fake kafka publisher
│   │   │   └── state.go              OrderState enum (PENDING, AWAITING_PAYMENT, …, COMPENSATED)
│   │   ├── kafka/
│   │   │   ├── producer.go           Publishes: OrderCreated, CompensatePayment
│   │   │   └── consumer.go           Consumes: PaymentProcessed, PaymentFailed, PaymentRefunded
│   │   └── store/
│   │       ├── store.go              OrderStore interface
│   │       └── memory.go             sync.RWMutex-guarded map implementation
│   │
│   ├── payment/
│   │   ├── kafka/
│   │   │   ├── consumer.go           Consumes: OrderCreated, CompensatePayment; idempotency map
│   │   │   └── producer.go           Publishes: PaymentProcessed, PaymentFailed, PaymentRefunded
│   │   └── ledger/
│   │       ├── ledger.go             PaymentLedger interface
│   │       └── memory.go             In-memory ledger; processPayment + refundPayment
│   │
│   └── inventory/
│       ├── grpc/
│       │   └── server.go             InventoryServiceServer implementation; reads INVENTORY_FAIL_ITEM_PREFIX
│       └── store/
│           ├── store.go              StockStore interface
│           └── memory.go             In-memory stock map; reserveStock + releaseStock
│
├── proto/                            single source of truth for gRPC contracts (FR-6)
│   ├── buf.yaml                      [committed] buf workspace / lint config
│   ├── buf.gen.yaml                  [committed] code-gen targets → internal/gen/
│   └── inventory/
│       └── v1/
│           └── inventory.proto       [committed] package inventory.v1; ReserveInventory + ReleaseInventory
│
├── deploy/
│   ├── docker-compose/
│   │   ├── docker-compose.yml        [committed] full stack: 3 services + Kafka + observability
│   │   └── .env.example              [committed] documents required env vars (copy to .env locally)
│   │
│   ├── kubernetes/                   plain YAML manifests; apply with: kubectl apply -f deploy/kubernetes/
│   │   ├── namespace.yaml            Namespace: ordersagademo
│   │   ├── order/
│   │   │   ├── deployment.yaml       Deployment: order-service
│   │   │   ├── service.yaml          Service: order-service (ClusterIP :8080)
│   │   │   └── configmap.yaml        ConfigMap: order-service-config (env vars)
│   │   ├── payment/
│   │   │   ├── deployment.yaml       Deployment: payment-service
│   │   │   └── configmap.yaml        ConfigMap: payment-service-config
│   │   └── inventory/
│   │       ├── deployment.yaml       Deployment: inventory-service
│   │       ├── service.yaml          Service: inventory-service (ClusterIP :9090)
│   │       └── configmap.yaml        ConfigMap: inventory-service-config
│   │
│   └── helm/
│       └── ordersagademo/            Helm chart root
│           ├── Chart.yaml            name: ordersagademo; kubeVersion: ">=1.28.0"; appVersion: "0.1.0"
│           ├── values.yaml           image.tag, replicaCount, kafka.bootstrapServers, otel.endpoint
│           └── templates/
│               ├── _helpers.tpl      shared label helpers
│               ├── namespace.yaml
│               ├── order/
│               │   ├── deployment.yaml
│               │   ├── service.yaml
│               │   └── configmap.yaml
│               ├── payment/
│               │   ├── deployment.yaml
│               │   └── configmap.yaml
│               └── inventory/
│                   ├── deployment.yaml
│                   ├── service.yaml
│                   └── configmap.yaml
│
├── observability/
│   ├── otelcollector/
│   │   └── otelcol-config.yaml       receivers: otlp; exporters: prometheusremotewrite, otlp/tempo, loki
│   │
│   ├── prometheus/
│   │   ├── prometheus.yml            scrape_configs: otel-collector :8889; self-scrape
│   │   └── rules/
│   │       └── saga-alerts.yaml      SagaCompensationRateHigh + PaymentFailureRateHigh rules
│   │
│   ├── loki/
│   │   └── loki-config.yaml          filesystem storage; single-binary mode
│   │
│   ├── tempo/
│   │   └── tempo-config.yaml         receivers: otlp; storage: local; retention: 1h (demo)
│   │
│   └── grafana/
│       ├── provisioning/
│       │   ├── datasources/
│       │   │   └── datasources.yaml  Prometheus + Loki + Tempo datasources with stable UIDs
│       │   └── dashboards/
│       │       └── provider.yaml     dashboard provider pointing to /var/lib/grafana/dashboards/
│       └── dashboards/
│           ├── saga-overview.json    throughput, compensation counter, step latency panels
│           └── slo-dashboard.json    order success rate + error-budget panels (AC-6, NFR-6)
│
├── scripts/
│   ├── create-order.sh               happy-path trigger: POST /orders with valid items
│   └── create-order-fail.sh          forced-failure trigger: POST /orders with item_id "FAIL_ITEM_01"
│
└── docs/
    ├── requirements/
    │   └── SRS.md                    [committed] existing
    ├── tech-stack.md                 [committed] this iteration
    └── design/
        ├── architecture.md           [committed] this iteration
        └── project-layout.md         [committed] this file
```

---

## Key File Responsibilities (Quick Reference)

### `go.mod`
```
module github.com/vladiant/ordersagademo
go 1.22
```
All three service binaries and all internal packages live in this single module. Third-party dependencies declared here.

### `Makefile` Target Summary
See `docs/tech-stack.md` §11 for the full target contract. The Developer must implement all listed targets.

### `proto/buf.yaml`
Declares `lint` rules (DEFAULT) and `breaking` detection (FILE mode). No external dep on `protoc`.

### `proto/buf.gen.yaml`
Declares two plugins: `protoc-gen-go` and `protoc-gen-go-grpc`. Output mapped to `../internal/gen`.

### `deploy/docker-compose/docker-compose.yml`
Must define services: `order-service`, `payment-service`, `inventory-service`, `kafka`, `otel-collector`, `prometheus`, `loki`, `tempo`, `grafana`.  
Must define health-checks for `kafka` and all three application services (NFR-14).  
Must set `INVENTORY_FAIL_ITEM_PREFIX=FAIL_` on `inventory-service` to enable forced-failure by default in the demo Compose file.

### `observability/otelcollector/otelcol-config.yaml`
Minimum pipeline:
- **Receiver:** `otlp` (grpc :4317)
- **Processor:** `batch`, `memory_limiter`
- **Exporters:** `prometheusremotewrite` (→ Prometheus), `otlp/tempo` (→ Tempo), `loki` (→ Loki)
- The Collector also exposes its own metrics on `:8888` for self-monitoring.

### `observability/grafana/provisioning/datasources/datasources.yaml`
Must hard-code stable UIDs (e.g., `prometheus-uid`, `loki-uid`, `tempo-uid`) so that the Tempo → Loki derived field link in dashboard JSON does not break across Grafana restarts.

---

## Paths NOT in the Layout (Out of Scope)

The following are explicitly excluded from this iteration per the SRS:

- `.github/` — no CI/CD workflows (deferred to later iteration)
- `scripts/load-test.sh` — no performance testing
- `deploy/kubernetes/kafka/` — Kafka runs in Compose only; in Kubernetes, services point to an external or Helm-managed Kafka
- Any `ui/` or `frontend/` directory
- Any service mesh configuration (`istio/`, `linkerd/`)

---

*Design is ready for the Developer agent to implement.*
