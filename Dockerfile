# syntax=docker/dockerfile:1

# ---------- web ----------
FROM node:20-alpine AS web
ARG NPM_REGISTRY=https://registry.npmjs.org
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm config set registry $NPM_REGISTRY && npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ---------- server ----------
FROM golang:1.26-alpine AS server
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/aidash ./cmd/server

# ---------- runtime ----------
# 面板自身以 root 运行：它需要访问 docker.sock、nsenter 进入宿主机命名空间，
# 这是「运维模式」的前提，与 d.run 平台组件在集群内的权限等级一致。
FROM alpine:3.20
ARG ALPINE_MIRROR=
RUN if [ -n "$ALPINE_MIRROR" ]; then sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${ALPINE_MIRROR}#g" /etc/apk/repositories; fi \
    && apk add --no-cache ca-certificates tzdata curl util-linux procps
WORKDIR /app
COPY --from=server /out/aidash /app/aidash
COPY --from=web /web/dist /app/web

ENV PORT=9090 \
    STATIC_DIR=/app/web \
    AIDASH_DATA_DIR=/data \
    DB_PATH=/data/aidash.db \
    GIN_MODE=release \
    TZ=Asia/Shanghai \
    AIDASH_HOST_NS=1

RUN mkdir -p /data
VOLUME ["/data"]
EXPOSE 9090
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD curl -fsS http://127.0.0.1:9090/api/health || exit 1

ENTRYPOINT ["/app/aidash"]
