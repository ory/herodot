# herodot

[![Build Status](https://travis-ci.org/ory/herodot.svg?branch=master)](https://travis-ci.org/ory/herodot)
[![Coverage Status](https://coveralls.io/repos/github/ory/herodot/badge.svg?branch=master)](https://coveralls.io/github/ory/herodot?branch=master)

---

Herodot is a lightweight SDK for writing RESTful responses. You can compare it to [render](https://github.com/unrolled/render),
although it currently supports only JSON.

Herodot is used by [Hydra](https://github.com/ory/hydra) and servers millions of requests already.

## Installation

Herodot is versioned using [glide](https://github.com/Masterminds/glide) and works best with
[pkg/errors](https://github.com/pkg/errors). To install it, run

```
go get -u github.com/ory/herodot
```

## Examples

Using Herodot is straight forward, these examples will help you getting started.

### JSON

Herodot supplies an interface, so it's possible to write many outputs, such as XML and others. For now, JSON is supported.

#### Write responses

```go
var writer = herodot.NewJSONWriter(nil)

func GetHandler(rw http.ResponseWriter, r *http.Request) {
	writer.Write(rw, r, map[string]interface{}{
	    "key": "value"
	})
}

type MyStruct struct {
    Key string `json:"key"`
}

func GetHandlerWithStruct(rw http.ResponseWriter, r *http.Request) {
	writer.Write(rw, r, &MyStruct{Key: "value"})
}

func PostHandlerWithStruct(rw http.ResponseWriter, r *http.Request) {
	writer.WriteCreated(rw, r, "/path/to/the/resource/that/was/created", &MyStruct{Key: "value"})
}

func SomeHandlerWithArbitraryStatusCode(rw http.ResponseWriter, r *http.Request) {
	writer.WriteCode(rw, r, http.StatusAccepted, &MyStruct{Key: "value"})
}

func SomeHandlerWithArbitraryStatusCode(rw http.ResponseWriter, r *http.Request) {
	writer.WriteCode(rw, r, http.StatusAccepted, &MyStruct{Key: "value"})
}
```

#### Dealing with errors

```go
var writer = herodot.NewJSONWriter(nil)

func GetHandlerWithError(rw http.ResponseWriter, r *http.Request) {
    if err := someFunctionThatReturnsAnError(); err != nil {
        writer.WriteError(w, r, err)
        return
    }
    
    // ...
}

func GetHandlerWithErrorCode(rw http.ResponseWriter, r *http.Request) {
    if err := someFunctionThatReturnsAnError(); err != nil {
        writer.WriteErrorCode(w, r, http.StatusBadRequest, err)
        return
    }
    
    // ...
}
```