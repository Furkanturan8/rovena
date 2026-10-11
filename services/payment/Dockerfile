# Build context is the repository root: docker build -f services/payment/Dockerfile .
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY shared/go.mod shared/go.sum ./shared/
COPY services/payment/go.mod services/payment/go.sum ./services/payment/
RUN cd services/payment && go mod download

COPY shared ./shared
COPY services/payment ./services/payment
RUN cd services/payment && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /app/
USER nonroot:nonroot
EXPOSE 8085
ENTRYPOINT ["/app/api"]
