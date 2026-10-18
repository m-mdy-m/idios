package main

import (
	"context"
	"io"
	"os"

	"idios/internal/application"
	"idios/internal/cli"
	"idios/internal/storage/sqlite"
)

func main() {
	factory := func(path string) (*application.Service, io.Closer, error) {
		store, err := sqlite.OpenStore(path)
		if err != nil {
			return nil, nil, err
		}
		return application.New(store), store, nil
	}
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, factory))
}
