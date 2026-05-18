.PHONY: all build run test clean docker-build docker-up docker-down web-install web-dev web-build install-deps release deploy

all: build run

build:
	cd server && go build -ldflags="-s -w" -o devdash ./cmd/server

run:
	cd server && ./devdash

test:
	cd server && go test ./... -v -race

clean:
	rm -rf server/devdash server/agent dist/

docker-build:
	docker build -f docker/Dockerfile.server -t devdash:latest .
	docker build -f docker/Dockerfile.agent -t devdash-agent:latest .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

docker-build-prod:
	docker buildx build --platform linux/amd64,linux/arm64 \
		-f docker/Dockerfile.server \
		-t ghcr.io/$(GITHUB_USER)/devdash:latest \
		-t ghcr.io/$(GITHUB_USER)/devdash:v$(VERSION) \
		--push .

deploy-prod:
	docker-compose -f docker-compose.prod.yml up -d

web-install:
	cd web && npm install

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

install-deps:
	cd server && go mod download

release:
	./scripts/build.sh

lint-server:
	cd server && go vet ./... && gofmt -l . | head -20

lint-web:
	cd web && npx vue-tsc --noEmit
