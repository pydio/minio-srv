# Go GRPC [![License](https://img.shields.io/:license-apache-blue.svg)](https://opensource.org/licenses/Apache-2.0) [![GoDoc](https://godoc.org/github.com/micro/go-grpc?status.svg)](https://godoc.org/github.com/micro/go-grpc) [![Travis CI](https://api.travis-ci.org/micro/go-grpc.svg?branch=master)](https://travis-ci.org/micro/go-grpc) [![Go Report Card](https://goreportcard.com/badge/micro/go-grpc)](https://goreportcard.com/report/github.com/micro/go-grpc)

Go-GRPC is a micro based gRPC framework for microservices.

Go-GRPC provides a [go-micro.Service](https://godoc.org/github.com/micro/go-micro#Service) leveraging gRPC plugins for the client, server and transport. Go-GRPC shares the [go-micro](https://github.com/micro/go-micro) codebase, making it a pluggable gRPC framework for microservices. Everything works 
just like a go-micro service, using micro generated protobufs and defaulting to consul for service discovery.

Go-GRPC interoperates with standard gRPC services seamlessly including the [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway).

Find an example greeter service in [examples/greeter](https://github.com/micro/go-grpc/tree/master/examples/greeter).

## Writing a Micro Service

Initialisation of a go-grpc service is identical to a go-micro service. Which means you can swap out `micro.NewService` for `grpc.NewService` 
with zero other code changes.

```go
package main

import (
	"log"
	"time"

	"github.com/micro/go-grpc"
	"github.com/micro/go-micro"
	hello "github.com/micro/go-grpc/examples/greeter/server/proto/hello"

	"golang.org/x/net/context"
)

type Say struct{}

func (s *Say) Hello(ctx context.Context, req *hello.Request, rsp *hello.Response) error {
	log.Print("Received Say.Hello request")
	rsp.Msg = "Hello " + req.Name
	return nil
}

func main() {
	service := grpc.NewService(
		micro.Name("go.micro.srv.greeter"),
		micro.RegisterTTL(time.Second*30),
		micro.RegisterInterval(time.Second*10),
	)

	// optionally setup command line usage
	service.Init()

	// Register Handlers
	hello.RegisterSayHandler(service.Server(), new(Say))

	// Run server
	if err := service.Run(); err != nil {
		log.Fatal(err)
	}
}
```

## Writing a Micro Function

Functions are one time executing Services. They look identical to go-micro Services which means you can swap out `micro.NewFunction` for `grpc.NewFunction` 
with zero other code changes.

```go
package main

import (
	"log"

	"github.com/micro/go-grpc"
	hello "github.com/micro/go-grpc/examples/greeter/server/proto/hello"
	"github.com/micro/go-micro"

	"golang.org/x/net/context"
)

type Say struct{}

func (s *Say) Hello(ctx context.Context, req *hello.Request, rsp *hello.Response) error {
	rsp.Msg = "Hello " + req.Name
	return nil
}

func main() {
	fn := grpc.NewFunction(
		micro.Name("go.micro.fnc.greeter"),
	)

	fn.Init()

	fn.Handle(new(Say))

	// Run server
	if err := fn.Run(); err != nil {
		log.Fatal(err)
	}
}
```

## Using with Micro toolkit

You may want to use the micro toolkit with grpc services. To do this either use the prebuilt toolkit or 
simply include the grpc client plugin and rebuild the toolkit.

### Go Get

```
go get github.com/micro/grpc/cmd/micro
```

### Build Yourself

```
go get github.com/micro/micro
```

Create a plugins.go file
```go
package main

import _ "github.com/micro/go-plugins/client/grpc"
import _ "github.com/micro/go-plugins/server/grpc"
```

Build binary
```shell
// For local use
go build -i -o micro ./main.go ./plugins.go
```

Flag usage of plugins
```shell
micro --client=grpc --server=grpc
```

## Using with gRPC Gateway

Go-GRPC seamlessly integrates with the gRPC ecosystem. This means the grpc-gateway can be used as per usual.

Find an example greeter api at [examples/grpc/gateway](https://github.com/micro/examples/tree/master/grpc/gateway).
