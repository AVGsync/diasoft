FROM golang:1.25-alpine AS builder

ARG BUILD_HTTP_PROXY
ARG BUILD_HTTPS_PROXY
ARG BUILD_NO_PROXY

ENV HTTP_PROXY=${BUILD_HTTP_PROXY} \
    HTTPS_PROXY=${BUILD_HTTPS_PROXY} \
    http_proxy=${BUILD_HTTP_PROXY} \
    https_proxy=${BUILD_HTTPS_PROXY} \
    NO_PROXY=${BUILD_NO_PROXY} \
    no_proxy=${BUILD_NO_PROXY}

WORKDIR /src

COPY go.mod go.sum ./
COPY vendor ./vendor
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o /out/gateway-service ./cmd/apiserver

FROM alpine:3.22

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /app

COPY --from=builder /out/gateway-service /app/gateway-service
COPY configs /app/configs

EXPOSE 8080

ENTRYPOINT ["/app/gateway-service", "-config-path", "/app/configs/apiserver.toml"]