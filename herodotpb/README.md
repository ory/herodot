# Herodot protobuf

The `github.com/ory/herodot/herodotpb` Go module contains the Herodot protobuf
schema, generated message types, and annotation extensions. It is versioned
independently of the `github.com/ory/herodot/v2` runtime.

Import the Go types with:

```go
import "github.com/ory/herodot/herodotpb"
```

Catalogs import `ory/herodot/v1/herodot.proto` and use the `ory.herodot.v1`
annotation namespace. Go v1 modules have no major-version suffix.

Run `make gen` from the repository root to regenerate the protobuf types and the
runtime's code table. Run `GOWORK=off go test ./...` from this directory to
check the protobuf module independently.

## Releases

Publish this module with tags such as `herodotpb/v1.0.0`.

Release the version required by `v2/go.mod` before releasing the runtime. The
first release is `herodotpb/v1.0.0`; local development resolves it through
`go.work` until it is published.

Update the runtime dependency in a follow-up PR with
`GOWORK=off go get github.com/ory/herodot/herodotpb@<version>` from `v2`, then
run `GOWORK=off go mod tidy`. Remove the initial `herodotpb` replacement from
`go.work` once `v1.0.0` is published. The runtime release process updates the
root module's dependency metadata when releasing herodot/v2.
