# A Protobuf Type Server

[![Go Documentation](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)][godocs]
![GitHub Tag](https://img.shields.io/github/v/tag/tierklinik-dobersberg/pbtype-server)

[godocs]: http://godoc.org/github.com/tierklinik-dobersberg/pbtype-server

A type server for protobuf file descriptors mainly used to work with `google.protobuf.Any` when pre-compiling the the proto files into the final binary is not acceptable or impossible.

The type-server implements a Connect-RPC/gRPC interface similar to the gRPC Server-Reflection and is defined in [tierklinik-dobersberg/apis](https://github.com/tierklinik-dobersberg/apis/blob/main/proto/tkd/typeserver/v1/typeserver.proto).

It also includes support for protobuf well-known types (`google/protobuf/*`).

## Usage

Just start the type-server binary and configure all protobuf sources that should be served using the command line flags `--source`. pbtype-server will automaticall re-fetch the protobuf sources ever 10 minutes. To configure the interval, use the `--interval <duration>` flag.

```bash
go build ./cmds/pbtype-server

./pbtype-server --interval 10m \
    --source github.com/tierklinik-dobersberg/apis.git//proto \
    --source github.com/bufbuild/protovalidate.git//proto/protovalidate
```

Sources are downloaded using the awesome [hashicorp/go-getter](https://github.com/hashicorp/go-getter) library which supports downloading from various sources and supports automatic unpacking of archives. Refer to it's documentation on how to specify URLs. 

## Client Library

This package also provides a simple Go client library for fetching protobuf type definitions:

```go
package main

import (
    "log"

    "github.com/tierklinik-dobersberg/pbtype-server/pkg/resolver"
    "github.com/tierklinik-dobersberg/pbtype-server/pkg/protoresolve"

    "google.golang.org/protobuf/reflect/protoreflect"
    "google.golang.org/protobuf/types/dynamicpb"
    "google.golang.org/protobuf/encoding/protojson"
    "google.golang.org/protobuf/reflect/protoregistry"
)

// The address of your type-server
const server = "https://types.dobersberg.vet"

func main() {
    resolver := resolver.New(server)

    // Find a message descriptor by it's fully qualified name
    desc, err := resolver.FindDescriptorByName("google.protobuf.Duration")
    if err != nil {
        log.Fatalf("failed to find descriptor: %s", err)
    }

    messageDescriptor, ok := desc.(protoreflect.MessageDescriptor)
    if !ok {
        log.Fatalf("expected a message descriptor but got %T", desc)
    }

    // do something with messageDescriptor
    // for example, create a new message using dynamicpb
    msg := dynamicpb.NewMessage(messageDescriptor)


    // Instead of using dynamicpb, one can also directly use NewMessage or NewMessageFromBytes
    msg, err := resolver.NewMessage("google.protobuf.Timestamp")


    // Also, resolver.Resolver implements protodesc.Resolver, protoreflect.MessageTypeResolver and protoreflect.ExtensionTypeResolver
    // so it's easy to use protojson on unknown types by specifying a custom resolver:
    encoder := &protojson.MarshalOptions{
        Resolver: resolver,
    }

    // The resolver will now query the type server if msg is unknown
    blob, err := encoder.Marshal(msg)

    // Instead of using resolver.New, which uses a new File and Type registry, it
    // is also possible to wrap protoregistry.GlobalFiles and protoregistry.GlobalTypes.
    // This way, all descriptors fetched by the resolver will be persisted in the 
    // global protoregistry.
    resolver := resolver.Wrap(
        server,
        protoregistry.GlobalFiles,
        protoregistry.GlobalTypes,
    ) 

    // If you don't want to populate/store descriptors in the global protoregistry
    // but still want your pre-compiled protobuf files to be available in the resolver
    // you can use the provided protoresolve package:

    encoder = &protojson.MarshalOptions{
        // Use a combined resolver which will query each resolver in order until
        // one is able to find the requested descriptor.
        // In this example, the pre-compiled proto files will be queries first
        // and only if no descriptor is found the resolver will ask the type-server.
        Resolver: protoresolve.NewCombinedResolver(
            protoresolve.NewGlobalResolver(), // provides access to protoregistry.GlobalFiles and protoregistry.GlobalTypes,
            resolver.New(server),
        ),
    }
}
```