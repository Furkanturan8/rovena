# Build context is the repository root: docker build -f services/inventory/Dockerfile .
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY shared/go.mod shared/go.sum ./shared/
COPY services/inventory/go.mod services/inventory/go.sum ./services/inventory/
RUN cd services/inventory && go mod download

COPY shared ./shared
COPY services/inventory ./services/inventory
RUN cd services/inventory && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /app/
USER nonroot:nonroot
EXPOSE 8083
ENTRYPOINT ["/app/api"]
