.PHONY: build test lint generate docs

build: docs
	go build -o bin/server ./cmd/server
	go build -o bin/db ./cmd/db

test:
	go test ./...

lint:
	go vet ./...

generate:
	sqlc generate

docs:
	hugo --minify --source site --destination ../web/sitepublic
