.PHONY: run build test lint fmt web deploy setup

run:
	go run ./cmd/api

build:
	go build -o bin/s1ngo ./cmd/api

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

web:
	cd web \&\& npm install \&\& npm run build
