# A Self-Documenting Makefile: http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html

# Project variables
PACKAGE = $(shell echo $${PWD\#\#*src/})
BINARY_NAME ?= $(shell basename $$PWD)
DOCKER_IMAGE ?= banzaicloud/hollowtrees

# Build variables
BUILD_DIR ?= build
BUILD_PACKAGE = ${PACKAGE}/cmd/daemon
VERSION ?= $(shell git symbolic-ref -q --short HEAD || git describe --tags --exact-match)
COMMIT_HASH ?= $(shell git rev-parse --short HEAD 2>/dev/null)
BUILD_DATE ?= $(shell date +%FT%T%z)
LDFLAGS += -X main.version=${VERSION} -X main.commitHash=${COMMIT_HASH} -X main.buildDate=${BUILD_DATE}
export CGO_ENABLED ?= 0
export GOOS = $(shell go env GOOS)
ifeq (${VERBOSE}, 1)
ifeq ($(filter -v,${GOARGS}),)
	GOARGS += -v
endif
endif

# Docker variables
DOCKER_TAG ?= ${VERSION}

# Dependency versions
DEP_VERSION = 0.5.0
GOLANGCI_VERSION = 1.17.1
LICENSEI_VERSION = 0.1.0
PROTOC_VERSION = 3.6.1
GOLANG_VERSION = 1.11

# Add the ability to override some variables
# Use with care
-include override.mk

.PHONY: clean
clean: ## Clean the working area and the project
	rm -rf bin/ ${BUILD_DIR}/

bin/dep: bin/dep-${DEP_VERSION}
	@ln -sf dep-${DEP_VERSION} bin/dep
bin/dep-${DEP_VERSION}:
	@mkdir -p bin
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | INSTALL_DIRECTORY=bin DEP_RELEASE_TAG=v${DEP_VERSION} sh
	@mv bin/dep $@

.PHONY: build
build: ## Build a binary
ifeq (${VERBOSE}, 1)
	go env
endif
ifneq (${IGNORE_GOLANG_VERSION_REQ}, 1)
	@printf "${GOLANG_VERSION}\n$$(go version | awk '{sub(/^go/, "", $$3);print $$3}')" | sort -t '.' -k 1,1 -k 2,2 -k 3,3 -g | head -1 | grep -q -E "^${GOLANG_VERSION}$$" || (printf "Required Go version is ${GOLANG_VERSION}\nInstalled: `go version`" && exit 1)
endif

	@$(eval GENERATED_BINARY_NAME = ${BINARY_NAME})
	@$(if $(strip ${BINARY_NAME_SUFFIX}),$(eval GENERATED_BINARY_NAME = ${BINARY_NAME}-$(subst $(eval) ,-,$(strip ${BINARY_NAME_SUFFIX}))),)
	go build ${GOARGS} -tags "${GOTAGS}" -ldflags "${LDFLAGS}" -o ${BUILD_DIR}/${GENERATED_BINARY_NAME} ${BUILD_PACKAGE}

.PHONY: build-release
build-release: LDFLAGS += -w
build-release: build ## Build a binary without debug information

.PHONY: build-debug
build-debug: GOARGS += -gcflags "all=-N -l"
build-debug: BINARY_NAME_SUFFIX += debug
build-debug: build ## Build a binary with remote debugging capabilities

.PHONY: docker
docker: export GOOS = linux
docker: BINARY_NAME_SUFFIX += docker
docker: build-release ## Build a Docker image
	docker build --build-arg BUILD_DIR=${BUILD_DIR} --build-arg BINARY_NAME=${GENERATED_BINARY_NAME} -t ${DOCKER_IMAGE}:${DOCKER_TAG} -f Dockerfile.local .
ifeq (${DOCKER_LATEST}, 1)
	docker tag ${DOCKER_IMAGE}:${DOCKER_TAG} ${DOCKER_IMAGE}:latest
endif

.PHONY: check
check: test-all lint ## Run tests and linters

bin/go-junit-report:
	@mkdir -p bin
	GOBIN=${PWD}/bin/ go get -u github.com/jstemmer/go-junit-report

TEST_PKGS ?= ./...
TEST_REPORT_NAME ?= results.xml
.PHONY: test
test: TEST_REPORT ?= main
test: SHELL = /bin/bash
test: bin/go-junit-report ## Run tests
	@mkdir -p ${BUILD_DIR}/test_results/${TEST_REPORT}
	@set -o pipefail
	go test -v $(filter-out -v,${GOARGS}) ${TEST_PKGS} 2>&1 | tee >(bin/go-junit-report > ${BUILD_DIR}/test_results/${TEST_REPORT}/${TEST_REPORT_NAME})
	wait

.PHONY: test-all
test-all: ## Run all tests
	@${MAKE} GOARGS="${GOARGS} -run .\*" TEST_REPORT=all test

.PHONY: test-integration
test-integration: ## Run integration tests
	@${MAKE} GOARGS="${GOARGS} -run ^TestIntegration\$$\$$" TEST_REPORT=integration test

bin/golangci-lint: bin/golangci-lint-${GOLANGCI_VERSION}
	@ln -sf golangci-lint-${GOLANGCI_VERSION} bin/golangci-lint
bin/golangci-lint-${GOLANGCI_VERSION}:
	@mkdir -p bin
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b ./bin/ v${GOLANGCI_VERSION}
	@mv bin/golangci-lint $@

.PHONY: lint
lint: bin/golangci-lint ## Run linter
	bin/golangci-lint run

bin/licensei: bin/licensei-${LICENSEI_VERSION}
	@ln -sf licensei-${LICENSEI_VERSION} bin/licensei
bin/licensei-${LICENSEI_VERSION}:
	@mkdir -p bin
	curl -sfL https://raw.githubusercontent.com/goph/licensei/master/install.sh | bash -s v${LICENSEI_VERSION}
	@mv bin/licensei $@

.PHONY: license-check
license-check: bin/licensei ## Run license check
	bin/licensei check
	./scripts/check-header.sh

.PHONY: license-cache
license-cache: bin/licensei ## Generate license cache
	bin/licensei cache

protoc: ## Run protobuf generation
	protoc -I pkg/grpcplugin/proto/ pkg/grpcplugin/proto/event.proto --go_out=plugins=grpc:pkg/grpcplugin/proto

.PHONY: list
list: ## List all make targets
	@${MAKE} -pRrn : -f $(MAKEFILE_LIST) 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | egrep -v -e '^[^[:alnum:]]' -e '^$@$$' | sort

.PHONY: help
.DEFAULT_GOAL := help
help:
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# Variable outputting/exporting rules
var-%: ; @echo $($*)
varexport-%: ; @echo $*=$($*)
