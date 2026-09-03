.PHONY: run build test lint fmt deploy setup

run:
	go run ./cmd/api

build:
	go build -o bin/rapbattle ./cmd/api

test:
	go test ./...

lint:
	go vet ./...

fmt:
	gofmt -w .

setup:
	./scripts/setup.sh

deploy:
	./scripts/deploy.sh
