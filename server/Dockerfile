# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.24-alpine AS build
WORKDIR /src

# Cache module downloads
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static binaries: no cgo, so they run on a minimal runtime image.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/seed ./cmd/seed && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/simulate ./cmd/simulate

# golang-migrate CLI, used by the entrypoint to apply migrations on first start.
ARG MIGRATE_VERSION=v4.19.1
RUN go install -tags=postgres -trimpath github.com/golang-migrate/migrate/v4/cmd/migrate@${MIGRATE_VERSION}
RUN cp /go/bin/migrate /out/migrate

# ---- runtime stage ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S -g 10001 app && adduser -S -u 10001 -G app app

WORKDIR /app

COPY --from=build /out/api /app/api
COPY --from=build /out/seed /app/seed
COPY --from=build /out/simulate /app/simulate
COPY --from=build /out/migrate /app/migrate
COPY --from=build /src/cmd/migrate/migrations /app/migrations
COPY scripts/docker-entrypoint.sh /app/docker-entrypoint.sh

RUN mkdir -p /app/logs /app/media && chown -R app:app /app

USER app
ENTRYPOINT ["/app/docker-entrypoint.sh"]
EXPOSE 2021
