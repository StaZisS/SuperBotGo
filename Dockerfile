FROM golang:1.25-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /superbot ./cmd/bot

# ---
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /superbot .
COPY config.example.yaml ./config.yaml
COPY i18n/ ./i18n/
COPY migrations/ ./migrations/
COPY deployments/ ./deployments/

EXPOSE 8080

ENTRYPOINT ["./superbot"]
