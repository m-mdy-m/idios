package main

import (
	"context"
	"io"
	"os"

	"idios/clients/cli"
	"idios/internal/application"
	"idios/internal/storage/sqlite"
)

func main() {
	if len(os.Args) > 1 && isServerCommand(os.Args[1]) {
		os.Exit(runServerCommand(os.Args[1:], os.Stdout, os.Stderr))
	}
	factory := func(path string) (*application.Service, io.Closer, error) {
		store, err := sqlite.OpenStore(path)
		if err != nil {
			return nil, nil, err
		}
		return application.New(store), store, nil
	}
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, factory))
}
