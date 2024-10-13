package protoresolve

import (
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type Resolver interface {
	protodesc.Resolver
	protoregistry.MessageTypeResolver
	protoregistry.ExtensionTypeResolver
}

type RegistryResolver struct {
	*protoregistry.Files
	*protoregistry.Types
}

// NewGlobalResolver returns a Resolver that is baked by protoregistry.GlobalFiles
// and protoregistry.GloablTypes.
func NewGlobalResolver() Resolver {
	return &RegistryResolver{
		Files: protoregistry.GlobalFiles,
		Types: protoregistry.GlobalTypes,
	}
}

type CombinedResolvers []Resolver

// NewCombinedResolver returns a new combined resolver that queries each resolver in
// order.
func NewCombinedResolver(resolvers ...Resolver) CombinedResolvers {
	r := make(CombinedResolvers, len(resolvers))
	copy(r, resolvers)

	return r
}

func (c CombinedResolvers) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	for _, r := range c {
		res, err := r.FindDescriptorByName(name)
		if err == nil {
			return res, nil
		}
	}

	return nil, protoregistry.NotFound
}

func (c CombinedResolvers) FindMessageByName(name protoreflect.FullName) (protoreflect.MessageType, error) {
	for _, r := range c {
		res, err := r.FindMessageByName(name)
		if err == nil {
			return res, nil
		}
	}

	return nil, protoregistry.NotFound
}

func (c CombinedResolvers) FindMessageByURL(name string) (protoreflect.MessageType, error) {
	for _, r := range c {
		res, err := r.FindMessageByURL(name)
		if err == nil {
			return res, nil
		}
	}

	return nil, protoregistry.NotFound
}

func (c CombinedResolvers) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	for _, r := range c {
		res, err := r.FindFileByPath(path)
		if err == nil {
			return res, nil
		}
	}

	return nil, protoregistry.NotFound
}

func (c CombinedResolvers) FindExtensionByName(name protoreflect.FullName) (protoreflect.ExtensionType, error) {
	for _, r := range c {
		res, err := r.FindExtensionByName(name)
		if err == nil {
			return res, nil
		}
	}

	return nil, protoregistry.NotFound
}

func (c CombinedResolvers) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	for _, r := range c {
		res, err := r.FindExtensionByNumber(message, field)
		if err == nil {
			return res, nil
		}
	}

	return nil, protoregistry.NotFound
}

var _ Resolver = (*RegistryResolver)(nil)
var _ Resolver = (CombinedResolvers)(nil)
