package resolver

import (
	"context"
	"fmt"

	"github.com/bufbuild/connect-go"
	typeserverv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/typeserver/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/typeserver/v1/typeserverv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Resolver struct {
	cli typeserverv1connect.TypeResolverServiceClient
	reg *protoregistry.Files
}

func New(url string) *Resolver {
	return &Resolver{
		cli: typeserverv1connect.NewTypeResolverServiceClient(
			cli.NewInsecureHttp2Client(),
			url,
		),
		reg: &protoregistry.Files{},
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

func (h *Resolver) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	if res, err := protoregistry.GlobalFiles.FindFileByPath(path); err == nil {
		return res, err
	}

	if res, err := h.reg.FindFileByPath(path); err == nil {
		return res, nil
	}

	res, err := h.cli.ResolveType(context.Background(), connect.NewRequest(&typeserverv1.ResolveRequest{
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
	if res, err := protoregistry.GlobalFiles.FindDescriptorByName(name); err == nil {
		return res, nil
	}

	if res, err := h.reg.FindDescriptorByName(name); err == nil {
		return res, nil
	}

	res, err := h.cli.ResolveType(context.Background(), connect.NewRequest(&typeserverv1.ResolveRequest{
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

	h.reg.RegisterFile(desc)

	return desc, nil
}
