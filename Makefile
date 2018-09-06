EXECUTABLE ?= hollowtrees
IMAGE ?= banzaicloud/$(EXECUTABLE)
TAG ?= dev-$(shell git log -1 --pretty=format:"%h")

LD_FLAGS = -X "main.version=$(TAG)"
PACKAGES = $(shell go list ./... | grep -v /vendor/)

.PHONY: _no-target-specified
_no-target-specified:
	$(error Please specify the target to make - `make list` shows targets.)

.PHONY: list
list:
	@$(MAKE) -pRrn : -f $(MAKEFILE_LIST) 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | egrep -v -e '^[^[:alnum:]]' -e '^$@$$' | sort

LICENSEI_VERSION = 0.0.7
bin/licensei: ## Install license checker
	@mkdir -p ./bin/
	curl -sfL https://raw.githubusercontent.com/goph/licensei/master/install.sh | bash -s v${LICENSEI_VERSION}

.PHONY: license-check
license-check: bin/licensei ## Run license check
	@bin/licensei check

.PHONY: license-cache
license-cache: bin/licensei ## Generate license cache
	@bin/licensei cache

DEP_VERSION = 0.5.0
bin/dep:
	@mkdir -p ./bin/
	@curl https://raw.githubusercontent.com/golang/dep/master/install.sh | INSTALL_DIRECTORY=./bin DEP_RELEASE_TAG=v${DEP_VERSION} sh

.PHONY: vendor
vendor: bin/dep ## Install dependencies
	bin/dep ensure -vendor-only

all: clean deps fmt vet docker push

clean:
	go clean -i ./...

build:
	go build .

deps:
	go get ./...

fmt:
	go fmt $(PACKAGES)

vet:
	go vet $(PACKAGES)

docker:
	docker build --rm -t $(IMAGE):$(TAG) .

push:
	docker push $(IMAGE):$(TAG)

run-dev:
	. .env
	go run $(wildcard *.go)

clean-vendor:
	find -L ./vendor -type l | xargs rm -rf
