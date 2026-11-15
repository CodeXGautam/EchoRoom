# One image for Render, docker-compose and (later) Kubernetes.

# ---- build ----
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache modules separately so code changes don't re-download dependencies.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO off → fully static binary that runs on a distroless base.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---- run ----
# distroless/static: no shell, no package manager, runs as non-root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
ENV APP_ENV=production
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
