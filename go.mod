module github.com/ory/herodot

go 1.26.0

require (
	github.com/ory/herodot/v2 v2.0.0
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.12.1
	golang.org/x/sync v0.24.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261019180522-ce908d29299b
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/jandelgado/gcov2lcov v1.1.1 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/telemetry v0.0.0-20261021172232-6cd571d43fbb // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
)

tool (
	github.com/jandelgado/gcov2lcov
	golang.org/x/tools/cmd/goimports
)
