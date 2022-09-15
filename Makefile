format: tools node_modules
		.bin/goimports -w -local github.com/ory *.go . httputil
		npm exec -- prettier --write .

node_modules: package-lock.json
	npm ci
	touch node_modules

tools:
		GOBIN=$(shell pwd)/.bin/ go install github.com/ory/go-acc golang.org/x/tools/cmd/goimports github.com/jandelgado/gcov2lcov
