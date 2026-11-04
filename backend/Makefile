# Makefile for managing the Go microservices project

ifeq ($(OS),Windows_NT)
    IS_WINDOWS := 1
endif

# load environment variables from .env file if it exists
ifneq (,$(wildcard .env))
    include .env
    export
endif

CURRENT_DIR	:= $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
ROOT_DIR	:= $(dir $(realpath $(lastword $(MAKEFILE_LIST))))

SRCS_MK		:= admin/Makefile operator/Makefile
PYTHON ?= python3
HZ ?= hz
HZ_VERSION ?= v0.9.1

.PHONY: help gen build api openapi init all vendor dep test cover vet lint docker \
		register install-dev install-prod docker-up docker-down docker-libs pm2-deploy ts build_only \
		infra-build infra-up infra-down infra-verify infra-config infra-test-tenant models hz

# show environment variables
env:
	echo "CURRENT_DIR: $(CURRENT_DIR)"
	echo "ROOT_DIR: $(ROOT_DIR)"
	echo "SRCS_MK: $(SRCS_MK)"

# initialize develop environment
init: plugin cli

# install protoc plugin
plugin:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/envoyproxy/protoc-gen-validate@latest
	go install github.com/tx7do/go-wind-toolkit/protoc-gen-go-redact@latest
	go install github.com/tx7do/go-wind-toolkit/protoc-gen-typescript-http@latest

# install cli tools
cli:
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/google/gnostic@latest
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install github.com/cloudwego/hertz/cmd/hz@$(HZ_VERSION)

# download dependencies of module
dep:
	go mod download

# create vendor
vendor:
	go mod vendor

# run tests
test:
	go test ./...

# run coverage tests
cover:
	go test -v ./... -coverprofile=coverage.out

# run static analysis
vet:
	go vet

# run lint
lint:
	golangci-lint run

# generate code
gen: api openapi

# register a new CRUD module into the hand-written wiring (usage: make register ENTITY=product)
register:
	go run ./platform/tools/register -entity $(ENTITY)

# generate protobuf api go code
api:
	$(PYTHON) platform/tools/codegen.py api
	$(MAKE) models
	$(MAKE) hz

# Persistence fields and GORM tags are defined exclusively in operator model proto.
models:
	$(PYTHON) platform/tools/gen-models.py

# Update HTTP routes with the official hz tool and regenerate typed RPC bindings.
hz:
	HZ=$(HZ) $(PYTHON) platform/tools/codegen.py hz
	$(PYTHON) platform/tools/gatewaygen.py

# generate protobuf api OpenAPI v3 docs.
openapi:
	$(PYTHON) platform/tools/codegen.py openapi
	go run ./platform/tools/openapigen platform/pkg/apidocs/openapi.yaml

# generate protobuf api Typescript code
ts:
	$(PYTHON) platform/tools/codegen.py ts

# build all service applications
build: api openapi
	$(MAKE) build_only

# only build all service applications without generating api and openapi
build_only:
	$(MAKE) -C admin build_only
	$(MAKE) -C operator build_only

# generate & build all service applications
all:
	$(MAKE) build

# use docker compose to run backend services and all its dependency services like redis, mysql, etc.
compose-up:
	docker compose up -d --force-recreate

# use docker compose to restart backend services and all its dependency services like redis, mysql, etc.
compose-restart:
	docker compose restart

# use docker compose to down backend services and all its dependency services like redis, mysql, etc.
compose-down:
	docker compose down

# build docker image
docker:
	$(MAKE) -C admin docker
	$(MAKE) -C operator docker

# ============================================================================
# Script Commands - 脚本命令
# ============================================================================

# install development environment (Unix/Linux/macOS)
install-dev:
	echo "Installing development environment..."
	bash platform/scripts/env/install_unix_dev.sh

# install production environment (Unix/Linux/macOS)
install-prod:
	echo "Installing production environment..."
	bash platform/scripts/env/install_unix_prod.sh

# start all services with docker compose (application + dependencies)
docker-up:
	echo "Starting all services (application + dependencies)..."
ifdef IS_WINDOWS
	powershell -ExecutionPolicy Bypass -File platform/scripts/docker/full_deploy.ps1
else
	bash platform/scripts/docker/full_deploy.sh
endif

# start only dependency services with docker compose (without application)
docker-libs:
	echo "Starting dependency services only..."
ifdef IS_WINDOWS
	powershell -ExecutionPolicy Bypass -File platform/scripts/docker/libs_only.ps1
else
	bash platform/scripts/docker/libs_only.sh
endif

# stop all docker compose services
docker-down:
	echo "Stopping all services..."
	docker compose down

# deploy services with PM2
pm2-deploy:
	echo "Deploying services with PM2..."
	bash platform/scripts/deploy/pm2_service.sh

# Build and deploy only admin/operator against the existing infra-docker network.
infra-build:
	docker compose --env-file .env.infra -f docker-compose.infra.yaml build
infra-config:
	$(PYTHON) platform/scripts/deploy/publish-infra-nacos.py
infra-up:
	docker compose --env-file .env.infra -f docker-compose.infra.yaml up -d --wait --wait-timeout 180
infra-down:
	docker compose --env-file .env.infra -f docker-compose.infra.yaml down
infra-verify:
	$(PYTHON) platform/scripts/deploy/verify-infra.py
infra-test-tenant:
	$(PYTHON) -m unittest discover -s platform/scripts/test -p 'test_tenant_lifecycle.py'
	$(PYTHON) platform/scripts/test/tenant_lifecycle.py
	$(PYTHON) platform/scripts/test/tenant_permissions.py

# show help
help:
	echo ""
	echo "Usage:"
	echo " make [target]"
	echo ""
	echo "Targets:"
	awk '/^[a-zA-Z\-_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
