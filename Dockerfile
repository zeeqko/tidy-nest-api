# Build context is this repo (backend/), e.g.:
#   docker build -t tidy-nest-api:local .
FROM golang:1.26-alpine AS backend-build
WORKDIR /backend
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
# CGO_ENABLED=0: static binary with no libc dependency, so it runs on the
# distroless base below (no shell, no package manager, minimal attack surface).
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- runtime ------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=backend-build /out/server ./server

# API-only image: the client repo ships its own image (nginx serving the
# built SPA) and the k8s Ingress path-routes "/" straight to it, bypassing
# this container entirely. STATIC_DIR is deliberately left unset — with no
# client/dist here, router.go's SPA catch-all route ("/*") would 404 if
# anything ever reached it, which nothing should. See k8s/README.md.

# Migrations and seed SQL are go:embed'd into the binary at build time (see
# internal/db/db.go) — nothing else needs to be copied in for those.

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
