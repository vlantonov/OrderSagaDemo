# Build from repo root: docker build -f services/payment/Dockerfile . (R-5)
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /payment ./cmd/payment

FROM gcr.io/distroless/static-debian12
COPY --from=builder /payment /payment
ENTRYPOINT ["/payment"]
