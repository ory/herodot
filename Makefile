SHELL=/bin/bash -o pipefail

.PHONY: format
format: tools
		goimports -w -local github.com/ory *.go . httputil

.PHONY: tools
tools:
		go install github.com/ory/go-acc golang.org/x/tools/cmd/goimports github.com/jandelgado/gcov2lcov
