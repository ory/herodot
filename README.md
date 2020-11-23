# herodot

[![Join the chat at https://discord.gg/PAMQWkr](https://img.shields.io/badge/join-chat-00cc99.svg)](https://discord.gg/PAMQWkr)
[![Build Status](https://travis-ci.org/ory/herodot.svg?branch=master)](https://travis-ci.org/ory/herodot)
[![Coverage Status](https://coveralls.io/repos/github/ory/herodot/badge.svg?branch=master)](https://coveralls.io/github/ory/herodot?branch=master)

---

Herodot is a lightweight SDK for writing RESTful responses. It is comparable to [render](https://github.com/unrolled/render)
but provides easier error handling. The error model implements the well established
[Google API Design Guide](https://cloud.google.com/apis/design/errors). Herodot currently supports only JSON responses
but can be extended easily.

Herodot is used by [ORY Hydra](https://github.com/ory/hydra) and serves millions of requests already.

## Installation

Herodot is versioned using [go modules](https://blog.golang.org/using-go-modules) and works best with
[pkg/errors](https://github.com/pkg/errors). To install it, run

```
go get -u github.com/ory/herodot
```

## Upgrading

Tips on upgrading can be found in [UPGRADE.md](UPGRADE.md)

## Usage

Using Herodot is straightforward. The examples below will help you get started.

### JSON

Herodot supplies an interface, allowing to return errors in many data formats like XML and others. Currently, it supports only JSON.

#### Write responses

```go
var hd = herodot.NewJSONWriter(nil)

func GetHandler(rw http.ResponseWriter, r *http.Request) {
	// run your business logic here
	hd.Write(rw, r, map[string]interface{}{
	    "key": "value"
	})
}

type MyStruct struct {
	Key string `json:"key"`
}

func GetHandlerWithStruct(rw http.ResponseWriter, r *http.Request) {
	// business logic
	hd.Write(rw, r, &MyStruct{Key: "value"})
}

func PostHandlerWithStruct(rw http.ResponseWriter, r *http.Request) {
	// business logic
	hd.WriteCreated(rw, r, "/path/to/the/resource/that/was/created", &MyStruct{Key: "value"})
}

func SomeHandlerWithArbitraryStatusCode(rw http.ResponseWriter, r *http.Request) {
	// business logic
	hd.WriteCode(rw, r, http.StatusAccepted, &MyStruct{Key: "value"})
}

func SomeHandlerWithArbitraryStatusCode(rw http.ResponseWriter, r *http.Request) {
	// business logic
	hd.WriteCode(rw, r, http.StatusAccepted, &MyStruct{Key: "value"})
}
```

#### Dealing with errors

```go
var writer = herodot.NewJSONWriter(nil)

func GetHandlerWithError(rw http.ResponseWriter, r *http.Request) {
    if err := someFunctionThatReturnsAnError(); err != nil {
        hd.WriteError(w, r, err)
        return
    }
    
    // ...
}

func GetHandlerWithErrorCode(rw http.ResponseWriter, r *http.Request) {
    if err := someFunctionThatReturnsAnError(); err != nil {
        hd.WriteErrorCode(w, r, http.StatusBadRequest, err)
        return
    }
    
    // ...
}
```

### Errors

Herodot implements the error model of the well established
[Google API Design Guide](https://cloud.google.com/apis/design/errors). Additionally, it makes the fields `request` and `reason` available. A complete Herodot error response looks like this:

```json
{
  "error": {
    "code": 404,
    "status": "some-status",
    "request": "foo",
    "reason": "some-reason",
    "details": [
      { "foo":"bar" }
    ],
    "message":"foo"
  }
}
```

To add context to your errors, implement `herodot.ErrorContextCarrier`. If you only want to set the status code of errors
implement [herodot.StatusCodeCarrier](https://github.com/ory/herodot/blob/master/error.go#L22-L26).
