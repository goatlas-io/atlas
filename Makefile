BRANCH := $(shell bash .ci/branch)
SUMMARY := $(shell SEP="-" bash .ci/version)
VERSION := $(shell cat VERSION)
NAME := $(shell basename `pwd`)
MODULE := $(shell head -n1 go.mod | cut -f2 -d' ')

.PHONY: docs-build docs-serve build release snapshot

vendor:
	go mod vendor

docs-build:
	docker run --rm -it -p 8000:8000 -v ${PWD}:/docs squidfunk/mkdocs-material build

docs-serve:
	docke/r run --rm -it -p 8000:8000 -v ${PWD}:/docs squidfunk/mkdocs-material

build:
	SUMMARY=$(SUMMARY) VERSION=$(VERSION) BRANCH=$(BRANCH) goreleaser build

release:
	SUMMARY=$(SUMMARY) VERSION=$(VERSION) BRANCH=$(BRANCH) goreleaser release --skip-publish --rm-dist --skip-validate

snapshot:
	GORELEASER_CURRENT_TAG=$(SUMMARY) SUMMARY=$(SUMMARY) VERSION=$(VERSION) BRANCH=$(BRANCH) goreleaser release --snapshot --skip-publish --rm-dist