package registry

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"github.com/hashicorp/go-getter"
	"github.com/tierklinik-dobersberg/pbtype-server/pkg/protoresolve"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	_ "google.golang.org/protobuf/types/gofeaturespb" // link in packages that include the standard protos included with protoc.
	_ "google.golang.org/protobuf/types/known/anypb"
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

var (
	ErrPollingStarted = errors.New("polling already started")
)

type Registry struct {
	started  chan struct{}
	interval time.Duration
	sources  []string

	l        sync.RWMutex
	files    linker.Files
	resolver linker.Resolver
}

func New(interval time.Duration, sources []string) *Registry {
	return &Registry{
		interval: interval,
		sources:  sources,
		started:  make(chan struct{}),
	}
}

func (reg *Registry) FileContainingSymbol(name protoreflect.FullName) (protoreflect.FileDescriptor, error) {
	resolver := reg.getResolver()

	response, err := resolver.FindDescriptorByName(name)
	if err != nil {
		return nil, err
	}

	fd, ok := response.(protoreflect.FileDescriptor)
	if ok {
		return fd, nil
	}

	return response.ParentFile(), nil
}

func (reg *Registry) FileByFilename(name string) (protoreflect.FileDescriptor, error) {
	resolver := reg.getResolver()

	res, err := resolver.FindFileByPath(name)
	if err != nil && errors.Is(err, protoregistry.NotFound) {
		// fallback to the global files registry
		res, err = protoregistry.GlobalFiles.FindFileByPath(name)
	}

	return res, err
}

func (reg *Registry) FileContaingURL(url string) (protoreflect.FileDescriptor, error) {
	resolver := reg.getResolver()

	message, err := resolver.FindMessageByURL(url)
	if err != nil {
		return nil, err
	}

	return message.Descriptor().ParentFile(), nil
}

func (reg *Registry) getResolver() linker.Resolver {
	reg.l.RLock()
	defer reg.l.RUnlock()

	return protoresolve.NewCombinedResolver(
		reg.resolver,
		protoresolve.NewGlobalResolver(),
	)
}

func (reg *Registry) StartPolling(ctx context.Context) error {
	select {
	case <-reg.started:
		return ErrPollingStarted

	default:
		close(reg.started)
	}

	go func() {
		ticker := time.NewTicker(reg.interval)
		defer ticker.Stop()

		for {
			slog.Info(("updating protobuf sources"))
			reg.updateSources(ctx)

			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
			}
		}
	}()

	return nil
}

// TODO(ppacher): use context.Context
func (reg *Registry) updateSources(_ context.Context) {
	var (
		files       []string
		importPaths []string
	)

	for idx, arg := range reg.sources {
		tmpdir, err := os.MkdirTemp("", fmt.Sprintf("pbtypes-%d-", idx))

		if err != nil {
			log.Fatal(err.Error())
		}
		defer os.RemoveAll(tmpdir)

		entry := slog.With("source", arg, "destination", tmpdir)

		entry.Info("downloading proto file")
		if err := getter.Get(tmpdir, arg); err != nil {
			entry.Error("failed to download proto files", "error", err)

			return
		}

		importPaths = append(importPaths, tmpdir)

		// find all proto files in tmpdir
		fs.WalkDir(os.DirFS(tmpdir), ".", func(path string, d fs.DirEntry, err error) error {
			if filepath.Ext(path) == ".proto" {
				files = append(files, path)
			}

			return nil
		})
	}

	slog.Info("compiling protobuf sources", "paths", importPaths, "files", len(files))

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: importPaths,
		}),
	}

	compiledFiles, err := compiler.Compile(context.Background(), files...)
	if err != nil {
		log.Fatal(err.Error())
	}

	reg.l.Lock()
	defer reg.l.Unlock()

	reg.files = compiledFiles
	reg.resolver = compiledFiles.AsResolver()
}
