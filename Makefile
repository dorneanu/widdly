# Go flags
GO ?= go
GO_ENV := $(shell $(GO) env GOOS GOARCH)
GOOS ?= $(word 1,$(GO_ENV))
GOARCH ?= $(word 2,$(GO_ENV))
GOFLAGS ?= $(GOFLAGS:)

all: test build

build: lambda dynamodb flatfile

lambda:
	@echo "> Building lambda version (with dynamodb store)"
	@$(GO) build $(GOFLAGS) -tags "dynamodb lambda" -o widdly-lambda
	zip widdly-lambda.zip widdly-lambda index.html

dynamodb:
	@echo "> Building widdly with dynamodb support"
	@$(GO) build $(GOFLAGS) -tags dynamodb -o widdly-dynamodb

flatfile:
	@echo "> Building widdly with flatfile support"
	@$(GO) build $(GOFLAGS) -tags flatfile -o widdly-flatfile

test:
	go test -v -tags flatfile ./...
	go test -v -tags bolt ./...
	go test -v -tags dynamodb ./...

clean:
	@rm widdly-dynamodb widdly-lambda widdly-flatfile


.PHONY: all test clean build lambda dynamodb flatfile
