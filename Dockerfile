FROM node:20-alpine AS frontend-builder

WORKDIR /app/web/admin

COPY web/admin/package.json web/admin/package-lock.json ./
RUN npm ci

COPY web/admin/ ./
RUN npm run build

FROM golang:1.25-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /app/web/admin/dist ./web/admin/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /superbot ./cmd/bot

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
