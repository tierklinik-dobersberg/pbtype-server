package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/typeserver/v1/typeserverv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/server"
	"github.com/tierklinik-dobersberg/pbtype-server/internal/registry"
	"github.com/tierklinik-dobersberg/pbtype-server/internal/service"
)

func main() {
	var (
		listenAddress string
		sources       []string
		interval      time.Duration
	)

	root := &cobra.Command{
		Use:  "pbtype-server [url...]",
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			reg := registry.New(interval, sources)

			if err := reg.StartPolling(ctx); err != nil {
				slog.Error("failed to start polling sources", "error", err)
				os.Exit(-1)
			}

			srv := service.New(reg)

			serveMux := http.NewServeMux()

			path, handler := typeserverv1connect.NewTypeResolverServiceHandler(srv)
			serveMux.Handle(path, handler)

			h2srv, err := server.CreateWithOptions(listenAddress, serveMux)
			if err != nil {
				slog.Error("failed to parpare server", "error", err)
				os.Exit(-1)
			}

			if err := server.Serve(ctx, h2srv); err != nil {
				slog.Error("failed to serve", "error", err)
				os.Exit(-1)
			}
		},
	}

	flags := root.Flags()
	{
		flags.StringVar(&listenAddress, "listen", ":8081", "The address to listen")
		flags.StringSliceVar(&sources, "source", nil, "A list of proto sources")
		flags.DurationVar(&interval, "refresh-interval", time.Minute*10, "The refresh interval for proto sources")
	}

	if err := root.Execute(); err != nil {
		slog.Error("failed to start server", "error", err)
		os.Exit(-1)
	}
}
