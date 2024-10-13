package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	_ "github.com/bufbuild/protocompile"
	"github.com/maxott/go-repl"
	"github.com/spf13/cobra"
	"github.com/tierklinik-dobersberg/pbtype-server/resolver"
	"google.golang.org/protobuf/reflect/protoreflect"

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

type handler struct {
	*resolver.Resolver
	r *repl.Repl
}

func (h *handler) Prompt() string {
	return "> "
}

func (h *handler) Tab(_ string) string {
	return ""
}

func (h *handler) Eval(line string) string {
	var (
		desc protoreflect.Descriptor
		err  error
	)

	switch {
	case strings.Contains(line, "/"):
		desc, err = h.Resolver.FindFileByPath(line)

	default:
		desc, err = h.Resolver.FindDescriptorByName(protoreflect.FullName(line))
	}

	if err != nil {
		return err.Error()
	}

	return fmt.Sprintf("%s (%T)", desc.FullName(), desc)
}

func main() {
	var (
		server string
	)

	cmd := &cobra.Command{
		Use: "pbtypecli URL",
		Run: func(_ *cobra.Command, args []string) {
			h := &handler{
				Resolver: resolver.New(server),
			}

			h.r = repl.NewRepl(h)

			if err := h.r.Loop(); err != nil {
				slog.Error("failed to run repl", "error", err)
				os.Exit(-1)
			}
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "http://localhost:8081", "The address of the type server")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}
