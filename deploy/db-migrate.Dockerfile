FROM postgres:16-alpine

WORKDIR /workspace

RUN apk add --no-cache bash wget \
    && wget -qO- 'https://artifacts-cli.infisical.com/setup.apk.sh' | sh \
    && apk update \
    && apk add --no-cache infisical

COPY deploy/migrate.sh /migrate.sh
COPY deploy/infisical-entrypoint.sh /infisical-entrypoint.sh

RUN chmod +x /migrate.sh /infisical-entrypoint.sh

ENTRYPOINT ["/infisical-entrypoint.sh", "sh", "/migrate.sh"]
