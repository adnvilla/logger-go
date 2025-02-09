.PHONY: build

all: build

build:
	@go mod tidy
	@go build ./...