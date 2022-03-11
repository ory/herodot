SHELL=/bin/bash -o pipefail

export PATH := .bin:${PATH}

.PHONY: format
format: tools
		goimports -w -local github.com/ory *.go . httputil

.PHONY: tools
tools:
		GOBIN=$(shell pwd)/.bin/ go install github.com/ory/go-acc golang.org/x/tools/cmd/goimports github.com/jandelgado/gcov2lcov
