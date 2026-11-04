# Shared targets for the flat admin/ and operator/ applications.
SERVICE_NAME := $(notdir $(CURDIR))
ROOT_DIR := $(abspath ..)
VERSION ?= 1.0.0
SERVICE_PACKAGE ?= ./cmd/server

.PHONY: build build_only run api openapi ts models wire wire-check docker
build: build_only
build_only:
	go build -ldflags "-X main.version=$(VERSION)" -o ./bin/$(SERVICE_NAME) $(SERVICE_PACKAGE)
run:
	go run $(SERVICE_PACKAGE)
api:
	$(MAKE) -C $(ROOT_DIR) api
openapi:
	$(MAKE) -C $(ROOT_DIR) openapi
ts:
	$(MAKE) -C $(ROOT_DIR) ts
models:
	$(MAKE) -C $(ROOT_DIR) models
wire:
	$(MAKE) -C $(ROOT_DIR) wire
wire-check:
	$(MAKE) -C $(ROOT_DIR) wire-check
docker:
	docker build --build-arg SERVICE_NAME=$(SERVICE_NAME) --build-arg APP_VERSION=$(VERSION) -f $(ROOT_DIR)/Dockerfile $(ROOT_DIR)
