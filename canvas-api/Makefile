.PHONY: build run tidy test vet

build:
	CGO_ENABLED=0 go build -trimpath -o bin/canvas-api ./cmd/server

run:
	go run ./cmd/server

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...
