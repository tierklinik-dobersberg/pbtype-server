package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bufbuild/connect-go"
	typeserverv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/typeserver/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/typeserver/v1/typeserverv1connect"
	"github.com/tierklinik-dobersberg/pbtype-server/internal/registry"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type TypeServer struct {
	registry *registry.Registry

	typeserverv1connect.UnimplementedTypeResolverServiceHandler
}

func New(registry *registry.Registry) *TypeServer {
	return &TypeServer{
		registry: registry,
	}
}

func (srv *TypeServer) ResolveType(ctx context.Context, req *connect.Request[typeserverv1.ResolveRequest]) (*connect.Response[typeserverv1.ResolveResponse], error) {
	var (
		desc protoreflect.FileDescriptor
		err  error
	)

	switch v := req.Msg.Kind.(type) {
	case *typeserverv1.ResolveRequest_FileByFilename:
		slog.Info("resolving proto type", "filename", v.FileByFilename)
		desc, err = srv.registry.FileByFilename(v.FileByFilename)

	case *typeserverv1.ResolveRequest_FileContainingSymbol:
		slog.Info("resolving proto type", "symbol", v.FileContainingSymbol)
		desc, err = srv.registry.FileContainingSymbol(protoreflect.FullName(v.FileContainingSymbol))

	case *typeserverv1.ResolveRequest_FileContainingUrl:
		slog.Info("resolving proto type", "url", v.FileContainingUrl)
		desc, err = srv.registry.FileContaingURL(v.FileContainingUrl)

	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("no message kind specified"))
	}

	if err != nil {
		if errors.Is(err, protoregistry.NotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		return nil, err
	}

	fproto := protodesc.ToFileDescriptorProto(desc)

	blob, err := proto.Marshal(fproto)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&typeserverv1.ResolveResponse{
		OriginalRequest: req.Msg,
		MessageResponse: &typeserverv1.ResolveResponse_FileDescriptor{
			FileDescriptor: &typeserverv1.FileDescriptorResponse{
				FileDescriptorProto: blob,
			},
		},
	}), nil
}
