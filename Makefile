format: tools node_modules
	.bin/goimports -w -local github.com/ory *.go . httputil
	npm exec -- prettier --write .

licenses: .bin/licenses node_modules  # checks open-source licenses
	.bin/licenses

.bin/licenses: Makefile
	curl https://raw.githubusercontent.com/ory/ci/master/licenses/install | sh

node_modules: package-lock.json
	npm ci
	touch node_modules

tools:
	GOBIN=$(shell pwd)/.bin/ go install github.com/ory/go-acc golang.org/x/tools/cmd/goimports github.com/jandelgado/gcov2lcov
