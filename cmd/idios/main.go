package main

import (
	"context"
	"io"
	"os"

	"idios/internal/application"
	"idios/internal/cli"
	"idios/internal/storage/sqlite"
	"idios/internal/tuilaunch"
)

func main() {
	// Running `idios` opens the interactive workspace. Explicit arguments continue to
	// use the scriptable CLI, e.g. `idios version`, `idios issue list`, or `idios --json`.
	if len(os.Args) == 1 {
		if err := tuilaunch.Run(false); err != nil {
			_, _ = io.WriteString(os.Stderr, err.Error()+"\n")
			if err == tuilaunch.ErrNonInteractive {
				os.Exit(2)
			}
			os.Exit(1)
		}
		return
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
