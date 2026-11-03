BIN := $(CURDIR)/bin

BUF_VERSION                := v1.75.0
PROTOC_GEN_GO_VERSION      := v1.36.12
PROTOC_GEN_GO_GRPC_VERSION := v1.6.2

MONGO_TEST_URI ?= mongodb://localhost:27017

# Packages counted for coverage: everything except generated protobuf code and main.
COVER_PKGS    := ./app/...,./middleware/...,./config/...,./logz/...
COVER_PROFILE := coverage.out

.PHONY: tools proto lint run build test test-integration cover cover-html up down tidy clean

## tools: install code generators into ./bin (no global install needed)
tools:
	GOBIN=$(BIN) go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	GOBIN=$(BIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(BIN) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

## proto: lint and regenerate Go code from .proto files
proto:
	$(BIN)/buf lint
	$(BIN)/buf generate

lint:
	go vet ./...

## up / down: start or stop local MongoDB (docker compose)
up:
	docker compose up -d --wait

down:
	docker compose down

## run: start the server against MongoDB using config/config.yaml (run `make up` first)
run:
	go run .

build:
	go build -o $(BIN)/server .

## test: tests that need no database (MongoDB-backed tests are skipped)
test:
	go test -race ./...

## test-integration: all tests, including MongoDB-backed ones (run `make up` first)
test-integration:
	MONGO_TEST_URI=$(MONGO_TEST_URI) go test -race -count=1 ./...

## cover: all tests (including MongoDB-backed) with coverage, then a per-function report (run `make up` first)
cover:
	MONGO_TEST_URI=$(MONGO_TEST_URI) go test -count=1 -coverpkg=$(COVER_PKGS) -coverprofile=$(COVER_PROFILE) ./...
	go tool cover -func=$(COVER_PROFILE)

## cover-html: open the coverage report from `make cover` in a browser
cover-html:
	go tool cover -html=$(COVER_PROFILE)

tidy:
	go mod tidy

clean:
	rm -rf $(BIN)/server $(COVER_PROFILE)
