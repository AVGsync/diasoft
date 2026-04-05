FROM rust:1-bookworm AS builder

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

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        cmake \
        pkg-config \
        libssl-dev \
        libsasl2-dev \
        zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

COPY Cargo.toml Cargo.lock ./
COPY src ./src
COPY config ./config

RUN cargo build --release

FROM debian:bookworm-slim

ARG BUILD_HTTP_PROXY
ARG BUILD_HTTPS_PROXY
ARG BUILD_NO_PROXY

ENV HTTP_PROXY=${BUILD_HTTP_PROXY} \
    HTTPS_PROXY=${BUILD_HTTPS_PROXY} \
    http_proxy=${BUILD_HTTP_PROXY} \
    https_proxy=${BUILD_HTTPS_PROXY} \
    NO_PROXY=${BUILD_NO_PROXY} \
    no_proxy=${BUILD_NO_PROXY}

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl bash gnupg \
    && curl -1sLf 'https://artifacts-cli.infisical.com/setup.deb.sh' | bash \
    && apt-get update \
    && apt-get install -y --no-install-recommends infisical \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /src/target/release/crypto-engine /usr/local/bin/crypto-engine
COPY infisical-entrypoint.sh /app/infisical-entrypoint.sh

RUN chmod +x /app/infisical-entrypoint.sh

ENTRYPOINT ["/app/infisical-entrypoint.sh", "crypto-engine"]
