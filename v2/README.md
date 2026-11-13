# herodot/v2

Errors identified by a domain and a reason from a reviewed proto catalog,
classified by a transport-neutral code, and rendered by transports. The design
follows [AIP-193](https://google.aip.dev/193), but HTTP statuses are
finer-grained than AIP's code-to-HTTP mapping.

| Package                  | Purpose                                                             |
| ------------------------ | ------------------------------------------------------------------- |
| `herodot`                | `Code`, `Definition`, `Error`, `Annotate`, `From`. No transport.    |
| `httperr`                | Renders errors as HTTP responses, by default in the AIP JSON shape. |
| `grpcerr`                | Renders errors as gRPC statuses, and server interceptors.           |
| `cmd/protoc-gen-herodot` | Generates Go, JSON, TypeScript, Markdown and OpenAPI from catalogs. |

## Catalogs

A catalog is an enum. The enum names the domain; its values are reasons:

```proto
import "ory/herodot/v1/herodot.proto";

enum FlowReason {
  option (ory.herodot.v1.catalog).domain = "kratos.ory.com";

  FLOW_REASON_UNSPECIFIED = 0;
  FLOW_EXPIRED = 1 [(ory.herodot.v1.value).reason = {
    code: EXPIRED
    message: "The self-service flow has expired."
    metadata_keys: ["flowId"]
  }];
}
```

Catalogs disable buf's `ENUM_VALUE_PREFIX` lint rule, because the value name is
the reason. Generate them with:

```yaml
# buf.gen.yaml
version: v2
plugins:
  - local: ["go", "run", "github.com/ory/herodot/v2/cmd/protoc-gen-herodot"]
    out: .
    opt: [paths=source_relative] # Go, next to the catalog
  - local: ["go", "run", "github.com/ory/herodot/v2/cmd/protoc-gen-herodot"]
    out: gen
    opt: [lang=json, lang=ts, lang=md, lang=openapi] # one file per domain
```

## Usage

```go
// Deep in the call stack.
return kratoserr.FlowExpired.New(
	herodot.Metadata("flowId", flow.ID.String()),
	herodot.Cause(err),
)

// Higher up: add context without changing the classification.
return herodot.Annotate(err, herodot.Metadata("returnTo", returnTo))

// Anywhere above.
if kratoserr.FlowExpired.Matches(err) { /* ... */ }

// At the edge.
w := &httperr.Writer{Debug: devMode}
w.WriteError(rw, r, err)

grpc.NewServer(grpc.ChainUnaryInterceptor(grpcerr.UnaryServerInterceptor(devMode)))
```

Errors without a classification are rendered as `ory.com/INTERNAL`. Their text
never reaches the client unless debugging is enabled.

### Errors from other services

An error received from another service would otherwise be unclassified too.
Decode it to keep its identity: catalog definitions match the decoded error, and
returning it forwards it unchanged.

```go
_, err := kratosClient.GetIdentity(ctx, req)
if e, ok := grpcerr.FromError(err); ok {
	return e
}

resp, err := http.DefaultClient.Do(req)
// ...
if e, ok := httperr.Decode(resp); ok {
	return e
}
```

The code survives both transports. HTTP responses carry the canonical code and
the HTTP status, which determine it. gRPC statuses carry only the canonical
code, so `grpcerr` adds an `ory.herodot.v1.CodeInfo` detail for refined codes.

## Development

The code table is generated from `herodot.proto`. After changing a proto, run
`make gen` in the repository root. It runs buf as a Go tool from
`herodotpb/tools/go.mod`, a separate module that keeps buf's dependencies out of
herodot/v2. The schema and generated protobuf types live in the independent
[herodotpb module](../herodotpb). The repository's `go.work` links all three
modules for local development.

## Releasing

Run the [Release](../.github/workflows/release.yml) workflow with a herodot/v2
version, a herodot v1 version, or both. It runs `scripts/release.sh`, which:

1. computes the `go.sum` lines of the new herodot/v2 version from master, before
   the tag exists, with `tools/cmd/modproxy`;
2. bumps the root module to require it, in a commit on top of master;
3. tags that commit and pushes master and the tags atomically.

Every herodot/v2 release bumps the root module. A herodot v1 release requires
the root module to require the latest herodot/v2 release. Run
`scripts/release.sh` without `--push` for a local dry run.

### Protobuf error details

Attach standard or service-specific protobuf messages with `ProtoDetail`:

```go
err := serviceUnavailable.New(
    herodot.ProtoDetail(&errdetails.RetryInfo{
        RetryDelay: durationpb.New(time.Second),
    }),
)
```

`ProtoDetail` also works with `Annotate`. Messages are copied when attached;
`Error.ProtoDetails()` returns deep copies. Except for `DebugInfo`, these
details are public response data. Use `Cause` for internal diagnostics and
`Violation` for invalid fields. Attached `DebugInfo` is rendered only when debug
output is enabled, just like cause-derived debug information. `BadRequest` can
also be attached directly. Only `ErrorInfo` and Herodot's `CodeInfo` are
reserved because they define the error's identity and code; passing either or a
nil message to `ProtoDetail` panics. Adding convenience APIs for other detail
types does not reserve those types.

`Violation(field, description)` is a convenience for attaching a `BadRequest`
with one field violation. Full `BadRequest` messages are retained when decoding
and forwarding, including reasons and localized messages. Read the complete
merged `BadRequest` through `ProtoDetails()`. All `BadRequest` messages merge
into one detail, including messages attached through `ProtoDetail` or decoded
from either transport. Violations append in attachment order without
deduplication, and protobuf unknown fields are preserved during merging.
`ProtoDetails()` returns the merged `BadRequest` first, followed by other
details in attachment order.

The HTTP and gRPC renderers include additional details alongside `ErrorInfo`.
HTTP uses protobuf JSON encoding, including `@type` and well-known types such as
durations. A request ID supplied by the transport takes precedence over an
attached `RequestInfo`.

Decoders retain additional details whose protobuf types are registered in the
process. Import the generated Go package for service-specific details. Unknown
or malformed details are ignored. Decoded `DebugInfo` is retained but only
rendered when debug output is enabled. Renderers omit details that cannot be
encoded in their transport format.
