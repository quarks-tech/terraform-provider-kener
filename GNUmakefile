SHELL := /usr/bin/env bash

BINARY   := terraform-provider-kener
VERSION  ?= dev
GOBIN    := $(shell go env GOPATH)/bin

COMPOSE  := docker compose -f test/docker-compose.yml
KENER_ENDPOINT ?= http://localhost:3000

.PHONY: default
default: build

## build: compile the provider binary
.PHONY: build
build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

## install: build and install the provider into GOPATH/bin
.PHONY: install
install:
	go install -ldflags "-X main.version=$(VERSION)" .

## test: run unit tests (no live Kener required)
.PHONY: test
test:
	go test ./... -count=1

## testacc: run acceptance tests against a local Kener (auto-starts it, mints a token)
.PHONY: testacc
testacc: kener-up
	@token=$$(scripts/kener-token.sh); \
	echo "Running acceptance tests against $(KENER_ENDPOINT)"; \
	TF_ACC=1 KENER_ENDPOINT=$(KENER_ENDPOINT) KENER_API_TOKEN=$$token \
		go test ./internal/provider/... -v -count=1 -timeout 30m

## fmt: format Go and Terraform files
.PHONY: fmt
fmt:
	gofmt -s -w .
	@command -v terraform >/dev/null 2>&1 && terraform fmt -recursive ./examples || true

## lint: run golangci-lint (installs it on demand)
.PHONY: lint
lint:
	@command -v golangci-lint >/dev/null 2>&1 || \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	$(GOBIN)/golangci-lint run ./...

## docs: generate provider documentation with tfplugindocs
.PHONY: docs
docs:
	@command -v tfplugindocs >/dev/null 2>&1 || \
		go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
	$(GOBIN)/tfplugindocs generate --provider-name kener

## kener-up: start the local Kener test instance and wait for readiness
.PHONY: kener-up
kener-up:
	$(COMPOSE) up -d
	@echo -n "waiting for Kener"; \
	for i in $$(seq 1 60); do \
		if [ "$$(curl -s -o /dev/null -w '%{http_code}' $(KENER_ENDPOINT)/ 2>/dev/null)" = "200" ]; then echo " ready"; exit 0; fi; \
		echo -n "."; sleep 2; \
	done; echo " timed out"; exit 1

## kener-token: print a fresh API token for the running Kener test instance
.PHONY: kener-token
kener-token:
	@scripts/kener-token.sh

## kener-down: stop and remove the local Kener test instance and its data
.PHONY: kener-down
kener-down:
	$(COMPOSE) down -v

## help: list available targets
.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | awk -F': ' '{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
