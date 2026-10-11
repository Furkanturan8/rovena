# Build context is the repository root: docker build -f services/catalog/Dockerfile .
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY shared/go.mod shared/go.sum ./shared/
COPY services/catalog/go.mod services/catalog/go.sum ./services/catalog/
RUN cd services/catalog && go mod download

COPY shared ./shared
COPY services/catalog ./services/catalog
RUN cd services/catalog && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /app/
USER nonroot:nonroot
EXPOSE 8082
ENTRYPOINT ["/app/api"]
