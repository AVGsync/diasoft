FROM node:22-alpine AS builder

ARG BUILD_HTTP_PROXY
ARG BUILD_HTTPS_PROXY
ARG BUILD_NO_PROXY

ENV HTTP_PROXY=${BUILD_HTTP_PROXY} \
    HTTPS_PROXY=${BUILD_HTTPS_PROXY} \
    http_proxy=${BUILD_HTTP_PROXY} \
    https_proxy=${BUILD_HTTPS_PROXY} \
    NO_PROXY=${BUILD_NO_PROXY} \
    no_proxy=${BUILD_NO_PROXY} \
    NODE_OPTIONS=--dns-result-order=ipv4first

WORKDIR /app

COPY package.json ./
RUN npm install

COPY . .
RUN npm run build

FROM nginx:1.27-alpine

RUN rm -f /etc/nginx/conf.d/default.conf /etc/nginx/http.d/default.conf

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY nginx.conf /etc/nginx/http.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

RUN mkdir -p /run/nginx

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
