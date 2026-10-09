# Build context is the repository root: docker build -f services/identity/Dockerfile .
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY shared/go.mod shared/go.sum ./shared/
COPY services/identity/go.mod services/identity/go.sum ./services/identity/
RUN cd services/identity && go mod download

COPY shared ./shared
COPY services/identity ./services/identity
RUN cd services/identity && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /app/
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/app/api"]
