package main

import (
	"log"

	"github.com/spf13/cobra"
)

func main() {
	var (
		server string
	)

	cmd := &cobra.Command{
		Use:  "pbtypecli URL",
		Args: cobra.MinimumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "localhost:8081", "The address of the type server")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}
