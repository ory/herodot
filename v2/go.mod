module github.com/ory/herodot/v2

go 1.26.0

tool google.golang.org/protobuf/cmd/protoc-gen-go

require (
	github.com/bufbuild/protocompile v0.14.1
	github.com/stretchr/testify v1.12.1
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261019180522-ce908d29299b
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
)

require (
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)
