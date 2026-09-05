# Build from repo root: docker build -f services/inventory/Dockerfile . (R-5)
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /inventory ./cmd/inventory

FROM gcr.io/distroless/static-debian12
COPY --from=builder /inventory /inventory
EXPOSE 9090
ENTRYPOINT ["/inventory"]
