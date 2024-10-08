# A Protobuf Type Server

This repository is going to contain a protobuf type server implementation that allows any service
to query protobuf file, message and service descriptors for use in dynamic environments (mainly when working with `anypb.Any`/`google/protobuf/any.proto`).

It will be based on `bufbuild/protocompile` and is expected to support fetching .proto files from various different sources like archives, git repos, http URLs, ...

Stay tuned :)