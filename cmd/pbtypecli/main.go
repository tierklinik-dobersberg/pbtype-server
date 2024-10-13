package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/bufbuild/protocompile"
	"github.com/hashicorp/go-getter"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func main() {
	cmd := &cobra.Command{
		Use:  "pbtypecli URL",
		Args: cobra.MinimumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			var (
				files       []string
				importPaths []string
			)

			for idx, arg := range args {
				tmpdir, err := os.MkdirTemp("", fmt.Sprintf("pbtypes-%d-", idx))

				if err != nil {
					log.Fatal(err.Error())
				}
				defer os.RemoveAll(tmpdir)

				log.Printf("downloading %s into %s", arg, tmpdir)
				if err := getter.Get(tmpdir, arg); err != nil {
					log.Fatalf("failed to download: %s", err.Error())
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

			r, err := compiledFiles.AsResolver().FindDescriptorByName("tkd.idm.v1.Profile")
			if err != nil {
				log.Fatal(err.Error())
			}

			msg := dynamicpb.NewMessage(r.(protoreflect.MessageDescriptor))

			blob, err := (&protojson.MarshalOptions{
				UseProtoNames:     true,
				EmitDefaultValues: true,
				EmitUnpopulated:   true,
				Indent:            "   ",
			}).Marshal(msg)
			if err != nil {
				log.Fatal(err.Error())
			}

			log.Println(string(blob))
		},
	}

	if err := cmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}
