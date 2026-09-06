# Build from repo root: docker build -f services/order/Dockerfile . (R-5)
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /order ./cmd/order

FROM gcr.io/distroless/static-debian12
COPY --from=builder /order /order
EXPOSE 8080
ENTRYPOINT ["/order"]
