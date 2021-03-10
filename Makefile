SHELL=/bin/bash -o pipefail

.PHONY: format
format: tools
		goreturns -w -local github.com/ory $$(listx .)

.PHONY: tools
tools:
		go install github.com/ory/go-acc github.com/sqs/goreturns github.com/jandelgado/gcov2lcov
