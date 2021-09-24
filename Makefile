BRANCH := $(shell bash .ci/branch)
SUMMARY := $(shell bash .ci/version)
VERSION := $(shell cat VERSION)
NAME := $(shell basename `pwd`)
MODULE := $(shell head -n1 go.mod | cut -f2 -d' ')

.PHONY: docs-build docs-serve

vendor:
	go mod vendor

build:
	go build -ldflags "-X $(MODULE)/pkg/common.SUMMARY=$(SUMMARY) -X $(MODULE)/pkg/common.BRANCH=$(BRANCH) -X $(MODULE)/pkg/common.VERSION=$(VERSION)" -o $(NAME)

release: vendor
	go build -mod=vendor -ldflags "-X $(MODULE)/pkg/common.SUMMARY=$(SUMMARY) -X $(MODULE)/pkg/common.BRANCH=$(BRANCH) -X $(MODULE)/pkg/common.VERSION=$(VERSION)" -o $(NAME) .

run-%:
	go run -mod=vendor -ldflags "-X $(MODULE)/pkg/common.SUMMARY=$(SUMMARY) -X $(MODULE)/pkg/common.BRANCH=$(BRANCH) -X $(MODULE)/pkg/common.VERSION=$(VERSION)" main.go $*

docs-build:
	docker run --rm -it -p 8000:8000 -v ${PWD}:/docs squidfunk/mkdocs-material build

docs-serve:
	docker run --rm -it -p 8000:8000 -v ${PWD}:/docs squidfunk/mkdocs-material
