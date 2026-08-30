# syntax=docker/dockerfile:1

# =============================================================================
# base — shared dependency layer
# =============================================================================
FROM golang:1.26-alpine AS base
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download

# =============================================================================
# dev — hot-reload via Air (make app-dev)
# =============================================================================
FROM base AS dev
RUN go install github.com/air-verse/air@latest
COPY . .
EXPOSE 8080
CMD ["sh", "-c", "rm -f tmp/build-errors.log; air -c .air.toml"]

# =============================================================================
# builder — static production binary
# =============================================================================
FROM base AS builder
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/console ./cmd/console

# =============================================================================
# prod — minimal runtime image
# =============================================================================
FROM alpine:3.20 AS prod
RUN apk add --no-cache ca-certificates && \
    addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/bin/api ./api
COPY --from=builder /app/bin/console ./console
COPY --from=builder /app/internal/database/migrations ./internal/database/migrations
RUN mkdir -p storage/uploads storage/logs && chown -R appuser:appgroup /app
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["./api"]
