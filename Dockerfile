# MediGt server image — multi-stage build.
#
# Runtime is the Go API + sqlc-generated handlers + migrations + the
# `medigt` ops CLI. The Next.js frontend is built and served from a
# separate image (apps/web/Dockerfile) so the API can scale independently.

# --- Build stage ---
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache deps so source-only changes don't bust the layer.
COPY server/go.mod server/go.sum ./server/
RUN cd server && go mod download

# Source.
COPY server/ ./server/

# Embed build info so /api/health can surface version + commit.
ARG VERSION=dev
ARG COMMIT=unknown
RUN cd server && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o bin/server  ./cmd/server
RUN cd server && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o bin/medigt  ./cmd/medigt
RUN cd server && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w"                                                   -o bin/migrate ./cmd/migrate

# --- Runtime stage ---
FROM alpine:3.21

# ca-certificates: TLS to SGK / e-Nabız / TURKKEP endpoints.
# tzdata: Europe/Istanbul date math (Z reports, gün sonu, vb.)
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Europe/Istanbul

WORKDIR /app

COPY --from=builder /src/server/bin/server  ./server
COPY --from=builder /src/server/bin/medigt  ./medigt
COPY --from=builder /src/server/bin/migrate ./migrate
COPY server/migrations ./migrations
COPY docker/entrypoint.sh .
RUN sed -i 's/\r$//' entrypoint.sh && chmod +x entrypoint.sh

# OpenShift assigns a random non-root UID at runtime; pre-creating
# upload+data dirs as world-writable keeps the entrypoint idempotent.
RUN mkdir -p /app/data/uploads && chmod 0777 /app/data/uploads

EXPOSE 8088

ENTRYPOINT ["./entrypoint.sh"]
