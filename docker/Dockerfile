FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ .
RUN npm run build

FROM golang:1.23-alpine AS backend
WORKDIR /app/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o /app/webshell ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates bash tzdata util-linux
COPY --from=backend /app/webshell /usr/local/bin/webshell
COPY --from=frontend /app/web/dist /app/web/dist
RUN mkdir -p /data && chmod +x /usr/local/bin/webshell

EXPOSE 9090
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -qO- http://localhost:9090/api/health || exit 1

ENTRYPOINT ["webshell"]
