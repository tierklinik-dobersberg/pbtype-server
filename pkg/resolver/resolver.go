package resolver

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bufbuild/connect-go"
	typeserverv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/typeserver/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/typeserver/v1/typeserverv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
	"github.com/tierklinik-dobersberg/pbtype-server/pkg/protoresolve"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"

	_ "google.golang.org/protobuf/types/gofeaturespb" // link in packages that include the standard protos included with protoc.
	_ "google.golang.org/protobuf/types/known/apipb"
	_ "google.golang.org/protobuf/types/known/durationpb"
	_ "google.golang.org/protobuf/types/known/emptypb"
	_ "google.golang.org/protobuf/types/known/fieldmaskpb"
	_ "google.golang.org/protobuf/types/known/sourcecontextpb"
	_ "google.golang.org/protobuf/types/known/structpb"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	_ "google.golang.org/protobuf/types/known/typepb"
	_ "google.golang.org/protobuf/types/known/wrapperspb"
	_ "google.golang.org/protobuf/types/pluginpb"
)

type ClientFactory interface {
	Create() (typeserverv1connect.TypeResolverServiceClient, error)
}

type staticClientFactory struct {
	cli typeserverv1connect.TypeResolverServiceClient
}

func (s staticClientFactory) Create() (typeserverv1connect.TypeResolverServiceClient, error) {
	return s.cli, nil
}

type Resolver struct {
	factory ClientFactory
	reg     *protoregistry.Files
	types   *protoregistry.Types
}

func New(url string) *Resolver {
	return Wrap(
		url,
		&protoregistry.Files{},
		&protoregistry.Types{},
	)
}

func WrapFactory(factory ClientFactory, files *protoregistry.Files, types *protoregistry.Types) *Resolver {
	return &Resolver{
		factory: factory,
		reg:     files,
		types:   types,
	}
}

func Wrap(url string, files *protoregistry.Files, types *protoregistry.Types) *Resolver {
	return &Resolver{
		factory: staticClientFactory{
			cli: typeserverv1connect.NewTypeResolverServiceClient(
				cli.NewInsecureHttp2Client(),
				url,
			),
		},
		reg:   files,
		types: types,
	}
}

func (h *Resolver) NewMessage(fullName protoreflect.FullName) (proto.Message, error) {
	desc, err := h.FindDescriptorByName(fullName)
	if err != nil {
		return nil, err
	}

	mDesc, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, fmt.Errorf("%s is not a message name, got %T", fullName, desc)
	}

	return dynamicpb.NewMessage(mDesc), nil
}

func (h *Resolver) NewMessageFromBytes(fullName protoreflect.FullName, blob []byte) (proto.Message, error) {
	msg, err := h.NewMessage(fullName)
	if err != nil {
		return nil, err
	}

	if err := proto.Unmarshal(blob, msg); err != nil {
		return nil, err
	}

	return msg, nil
}

func (h *Resolver) UnpackAny(m *anypb.Any) (proto.Message, error) {
	name := m.TypeUrl

	if strings.Contains(name, "googleapis") {
		_, name, _ = strings.Cut(name, "/")
	}

	return h.NewMessageFromBytes(protoreflect.FullName(name), m.Value)
}

func (h *Resolver) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	if res, err := h.reg.FindFileByPath(path); err == nil {
		return res, nil
	}

	cli, err := h.factory.Create()
	if err != nil {
		return nil, err
	}
	res, err := cli.ResolveType(context.Background(), connect.NewRequest(&typeserverv1.ResolveRequest{
		Kind: &typeserverv1.ResolveRequest_FileByFilename{
			FileByFilename: path,
		},
	}))
	if err != nil {
		return nil, err
	}

	return h.parseFileDescriptorProto(res.Msg.GetFileDescriptor().GetFileDescriptorProto())
}

func (h *Resolver) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	if res, err := h.reg.FindDescriptorByName(name); err == nil {
		slog.Debug("found type in local registry", "name", name)

		return res, nil
	}

	slog.Info("trying to resolve type", "name", name)

	cli, err := h.factory.Create()
	if err != nil {
		return nil, err
	}
	res, err := cli.ResolveType(context.Background(), connect.NewRequest(&typeserverv1.ResolveRequest{
		Kind: &typeserverv1.ResolveRequest_FileContainingSymbol{
			FileContainingSymbol: string(name),
		},
	}))
	if err != nil {
		return nil, err
	}

	if _, err := h.parseFileDescriptorProto(res.Msg.GetFileDescriptor().GetFileDescriptorProto()); err != nil {
		return nil, err
	}

	return h.reg.FindDescriptorByName(name)
}

func (h *Resolver) parseFileDescriptorProto(blob []byte) (protoreflect.FileDescriptor, error) {
	parsed := new(descriptorpb.FileDescriptorProto)

	if err := proto.Unmarshal(blob, parsed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal file descriptor proto: %w", err)
	}

	desc, err := protodesc.NewFile(parsed, h)
	if err != nil {
		return nil, fmt.Errorf("failed to create file descriptor: %w", err)
	}

	// register the file at the registry
	h.reg.RegisterFile(desc)

	// also, register all message, enum and extension types
	for idx := 0; idx < desc.Extensions().Len(); idx++ {
		h.types.RegisterExtension(
			dynamicpb.NewExtensionType(desc.Extensions().Get(idx)),
		)
	}
	for idx := 0; idx < desc.Messages().Len(); idx++ {
		h.types.RegisterMessage(
			dynamicpb.NewMessageType(desc.Messages().Get(idx)),
		)
	}
	for idx := 0; idx < desc.Enums().Len(); idx++ {
		h.types.RegisterEnum(
			dynamicpb.NewEnumType(desc.Enums().Get(idx)),
		)
	}

	return desc, nil
}

func (h *Resolver) FindExtensionByName(name protoreflect.FullName) (protoreflect.ExtensionType, error) {
	return h.types.FindExtensionByName(name)
}

func (h *Resolver) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	return h.types.FindExtensionByNumber(message, field)
}

func (h *Resolver) FindMessageByName(name protoreflect.FullName) (protoreflect.MessageType, error) {
	_, err := h.FindDescriptorByName(name)
	if err != nil {
		return nil, err
	}

	return h.types.FindMessageByName(name)
}

func (h *Resolver) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	_, name, _ := strings.Cut(url, "/")

	_, err := h.FindDescriptorByName(protoreflect.FullName(name))
	if err != nil {
		return nil, err
	}

	return h.types.FindMessageByURL(url)
}

var _ protoresolve.Resolver = (*Resolver)(nil)
