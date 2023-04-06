format: .bin/ory tools node_modules
	.bin/ory dev headers copyright --type=open-source --exclude=httputil
	.bin/goimports -w -local github.com/ory *.go . httputil
	npm exec -- prettier --write .

licenses: .bin/licenses node_modules  # checks open-source licenses
	.bin/licenses

.bin/licenses: Makefile
	curl https://raw.githubusercontent.com/ory/ci/master/licenses/install | sh

.bin/ory: Makefile
	curl https://raw.githubusercontent.com/ory/meta/master/install.sh | bash -s -- -b .bin ory v0.2.2
	touch .bin/ory

node_modules: package-lock.json
	npm ci
	touch node_modules

tools:
	GOBIN=$(shell pwd)/.bin/ go install github.com/ory/go-acc golang.org/x/tools/cmd/goimports github.com/jandelgado/gcov2lcov
