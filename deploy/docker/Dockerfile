# syntax=docker/dockerfile:1
# File: deploy/docker/Dockerfile
# Usage: Builds both binaries (api and worker); the compose file picks which one runs via `entrypoint`.
# Generated code (oapi-codegen, sqlc) must be committed before building.
FROM golang:1.26-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/ /app/
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/api"]
