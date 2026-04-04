FROM node:22-alpine AS builder

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

ENV HTTP_PROXY=${HTTP_PROXY} \
    HTTPS_PROXY=${HTTPS_PROXY} \
    http_proxy=${HTTP_PROXY} \
    https_proxy=${HTTPS_PROXY} \
    NO_PROXY=${NO_PROXY} \
    no_proxy=${NO_PROXY} \
    NODE_OPTIONS=--dns-result-order=ipv4first

WORKDIR /app

COPY package.json ./
RUN npm install

COPY . .
RUN npm run build

FROM alpine:3.22

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

ENV HTTP_PROXY=${HTTP_PROXY} \
    HTTPS_PROXY=${HTTPS_PROXY} \
    http_proxy=${HTTP_PROXY} \
    https_proxy=${HTTPS_PROXY} \
    NO_PROXY=${NO_PROXY} \
    no_proxy=${NO_PROXY}

RUN apk add --no-cache nginx

COPY nginx.conf /etc/nginx/http.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

RUN mkdir -p /run/nginx

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
