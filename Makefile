format: .bin/ory node_modules
	.bin/ory dev headers copyright --type=open-source --exclude=httputil --exclude=v2/cmd/protoc-gen-herodot/testdata
	go tool goimports -w -local github.com/ory *.go . httputil $(shell find v2 -name "*.go" -not -name "*.pb.go")
	npm exec -- prettier --write .

BUF = GOWORK=off go tool -modfile=tools/go.mod buf

gen:  # regenerates herodot/v2 from its protos
	cd v2 && $(BUF) lint && $(BUF) generate

licenses: .bin/licenses node_modules  # checks open-source licenses
	.bin/licenses

.bin/licenses: Makefile
	curl --retry 7 --retry-connrefused https://raw.githubusercontent.com/ory/ci/master/licenses/install | sh

.bin/ory: Makefile
	curl --retry 7 --retry-connrefused https://raw.githubusercontent.com/ory/meta/master/install.sh | bash -s -- -b .bin ory v0.2.2
	touch .bin/ory

node_modules: package-lock.json
	npm ci
	touch node_modules
