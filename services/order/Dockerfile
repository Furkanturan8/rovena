# Build context is the repository root: docker build -f services/order/Dockerfile .
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY shared/go.mod shared/go.sum ./shared/
COPY services/order/go.mod services/order/go.sum ./services/order/
RUN cd services/order && go mod download

COPY shared ./shared
COPY services/order ./services/order
RUN cd services/order && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /app/
USER nonroot:nonroot
EXPOSE 8086
ENTRYPOINT ["/app/api"]
