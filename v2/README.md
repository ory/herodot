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
| `proto/ory/herodot/v1`   | The code table and catalog options (`herodot.proto`).               |

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
`make gen` in the repository root. It runs buf as a Go tool from `tools/go.mod`,
a separate module that keeps buf's dependencies out of herodot/v2. A `go.work`
file in the repository root links this module and the herodot v1 module, which
depends on it.

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
