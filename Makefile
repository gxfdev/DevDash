.PHONY: all build build-linux build-windows run test test-unit test-integration clean \
       docker-build docker-up docker-down docker-logs \
       docker-dev-up docker-dev-down \
       dev-start dev-stop dev-status dev-logs \
       deploy deploy-check deploy-rollback deploy-status \
       lint lint-server lint-web \
       security-scan \
       web-install web-dev web-build \
       install-deps release

all: build

build:
	cd server && go build -ldflags="-s -w" -trimpath -o devdash ./cmd/server

build-linux:
	@mkdir -p dist
	cd server && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o ../dist/devdash-linux-amd64 ./cmd/server

build-windows:
	@mkdir -p dist
	cd server && CGO_ENABLED=1 go build -ldflags="-s -w" -trimpath -o ../dist/devdash-windows-amd64.exe ./cmd/server

run:
	cd server && go run ./cmd/server

test:
	cd server && go test ./... -v -race -timeout 120s

test-unit:
	cd server && go test ./internal/... -v -race -timeout 120s -coverprofile=coverage.out

test-integration:
	cd server && go test ./tests/... -v -timeout 120s

clean:
	$(RM) -r server/devdash server/agent dist/ .pids/ .logs/ 2>/dev/null || true

lint: lint-server lint-web

lint-server:
	cd server && go vet ./...
	@output=$$(gofmt -l .); if [ -n "$$output" ]; then echo "Unformatted files:"; echo "$$output"; exit 1; fi

lint-web:
	cd web && npx vue-tsc --noEmit

security-scan:
	@which govulncheck > /dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	cd server && govulncheck ./...

docker-build:
	docker build -f docker/Dockerfile.server -t devdash:latest .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

docker-dev-up:
	docker compose -f docker-compose.dev.yml up -d --build

docker-dev-down:
	docker compose -f docker-compose.dev.yml down

dev-start:
	bash dev.sh start

dev-stop:
	bash dev.sh stop

dev-status:
	bash dev.sh status

dev-logs:
	bash dev.sh logs

deploy:
	bash deploy/deploy.sh

deploy-check:
	bash deploy/deploy.sh --check

deploy-rollback:
	bash deploy/backup.sh restore

deploy-status:
	systemctl status devdash 2>/dev/null || echo "Service not installed"

web-install:
	cd web && npm install

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

install-deps:
	cd server && go mod download
	cd web && npm ci

release:
	bash scripts/build.sh
