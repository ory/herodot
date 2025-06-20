module github.com/ory/herodot

go 1.24

require (
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.10.0
	golang.org/x/sync v0.15.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250528174236-200df99c418a
	google.golang.org/grpc v1.73.0
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/jandelgado/gcov2lcov v1.1.1 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/mod v0.25.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	golang.org/x/tools v0.33.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

tool (
	github.com/jandelgado/gcov2lcov
	golang.org/x/tools/cmd/goimports
)
