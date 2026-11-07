.PHONY: help build web server test demo fmt vet docker up down logs screenshots e2e

help:
	@echo "AIDash 常用命令："
	@echo "  make web         构建前端"
	@echo "  make server      构建后端"
	@echo "  make build       构建前后端"
	@echo "  make test        Go 测试（含 mock Docker 引擎集成测试）"
	@echo "  make demo        无 Docker 的演示模式（内存 Docker 引擎 + 演示数据）"
	@echo "  make docker      构建镜像"
	@echo "  make up          启动 docker compose"
	@echo "  make down        停止"
	@echo "  make e2e         Playwright UI 端到端验证"

build: web server

web:
	cd web && npm install --no-audit --no-fund && npm run build

server:
	cd server && go build ./...

test:
	cd server && go test ./...

fmt:
	cd server && gofmt -w .

vet:
	cd server && go vet ./...

demo:
	cd server && go run ./cmd/demo

docker:
	docker build -t aidash:latest .

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f aidash

screenshots:
	cd web && node scripts/shots.mjs

e2e:
	cd web && node scripts/e2e.mjs
