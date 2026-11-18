SHELL := /bin/bash

# Image tag: the short commit SHA; with uncommitted changes (tracked or untracked), plus a hash
# of those changes. A rebuild from a changed tree gets a new tag, so `helm upgrade` rolls it out
# (with the same tag it would be a no-op and the pod would keep the old image); the same tree
# always gets the same tag, so separate `make image` and `make deploy` runs agree.
GIT_SHA     := $(shell git rev-parse --short HEAD)
GIT_CHANGES := $(shell git status --porcelain)
GIT_DIFF_ID := $(shell { git diff HEAD; git ls-files --others --exclude-standard -z | xargs -0 git hash-object 2>/dev/null; } | git hash-object --stdin | cut -c1-7)
TAG         ?= $(GIT_SHA)$(if $(GIT_CHANGES),-dirty-$(GIT_DIFF_ID))

# `make run`: the compose PostgreSQL from distrocommerce-infra (local dev
# credentials, the same as its compose/.env.example). Override as needed.
LOCAL_DATABASE_URL ?= postgres://product:product_dev@localhost:5432/product?sslmode=disable
# Not 8080: kind holds 127.0.0.1:8080 for Envoy, and a local :8080 listener
# would take over localhost (::1) requests meant for the cluster.
LOCAL_PORT ?= 8082

.PHONY: build test run

build: ## binary in bin/
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(TAG)" -o bin/product-service ./cmd/product-service

test: ## vet + unit tests (set PRODUCT_TEST_DATABASE_URL to include the store test)
	go vet ./...
	go test -race ./...

run: ## locally on :8082, against localhost:5432, telemetry off
	PORT=$(LOCAL_PORT) DATABASE_URL='$(LOCAL_DATABASE_URL)' LOG_LEVEL=debug OTEL_SDK_DISABLED=true go run ./cmd/product-service
