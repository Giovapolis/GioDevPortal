.PHONY: run build sqlc tailwind test tidy docker-build docker-up docker-down

run:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -trimpath -o bin/giodevportal ./cmd/server

sqlc:
	sqlc generate

tailwind:
	npm run tailwind:build

tailwind-watch:
	npm run tailwind:watch

test:
	go test ./... -count=1

test-race:
	go test -race ./... -count=1

tidy:
	go mod tidy

docker-build:
	docker compose -f docker/docker-compose.yml build

docker-up:
	docker compose -f docker/docker-compose.yml up -d

docker-down:
	docker compose -f docker/docker-compose.yml down

fmt:
	gofmt -w ./cmd ./internal ./pkg
