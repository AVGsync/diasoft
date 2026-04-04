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

RUN GOBIN=/out go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.1

FROM postgres:16-alpine

WORKDIR /workspace

COPY --from=builder /out/migrate /usr/local/bin/migrate
COPY deploy/migrate-go.sh /migrate-go.sh

ENTRYPOINT ["sh", "/migrate-go.sh"]
