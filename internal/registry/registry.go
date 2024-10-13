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
	"google.golang.org/protobuf/reflect/protoreflect"
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

	return fd.ParentFile(), nil
}

func (reg *Registry) FileByFilename(name string) (protoreflect.FileDescriptor, error) {
	resolver := reg.getResolver()
	return resolver.FindFileByPath(name)
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

	return reg.resolver
}

func (reg *Registry) StartPolling(ctx context.Context) error {
	select {
	case <-reg.started:
		return ErrPollingStarted

	default:
	}

	go func() {
		ticker := time.NewTicker(reg.interval)
		defer ticker.Stop()

		for {
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
			entry.Error("failed to download proto files")

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
