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
