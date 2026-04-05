FROM postgres:16-alpine

WORKDIR /workspace

COPY deploy/migrate.sh /migrate.sh

ENTRYPOINT ["sh", "/migrate.sh"]
