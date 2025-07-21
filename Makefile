SHELL := /bin/bash

.DEFAULT_GOAL := all
BUILD_DIR := build

BINARIES := magonote magonote-tmux

COMMIT_SHA ?= $(shell git describe --tags --always --dirty)

GO_LDFLAGS = -ldflags "-X main.CommitSha=$(COMMIT_SHA)"

GO := go
GO_BUILD := $(GO) build
GO_TEST := $(GO) test
GO_FMT := $(GO) fmt
GO_LINT := golangci-lint

.PHONY: all test format clean e2e build $(BINARIES)

BUILD_DIR:
	mkdir -p $(BUILD_DIR)

$(BINARIES): | BUILD_DIR
	@echo "Building $@..."
	$(GO_BUILD) ${GO_LDFLAGS} -o $(BUILD_DIR)/$@ ./cmd/$@

build: $(BINARIES)

all: $(BINARIES)

clean:
	@echo "Cleaning binaries..."
	rm -rf $(BUILD_DIR)/*

test:
	@echo "Running go test..."
	@$(GO_TEST) -v ./...
	@exit_code=$$?; \
	if [ $$exit_code -eq 0 ]; then \
		echo "🎉 All tests passed!"; \
	else \
		echo "❌ Tests failed with exit code $$exit_code"; \
	fi; \
	exit $$exit_code

format:
	@$(GO_FMT) ./...

lint:
	@$(GO_LINT) run

e2e: $(BINARIES)
	@echo "Running automated E2E tests..."
	@cd test/e2e && go test -v -timeout=30s .

manual-e2e:
	@echo "Running manual E2E test fixtures..."
	@bash ./tools/manual-run-fixtures.sh