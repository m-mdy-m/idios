// Package tuilaunch wires the shared TUI to either the clean or demo workspace database.
package tuilaunch

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"idios/internal/application"
	"idios/internal/config"
	"idios/internal/storage/sqlite"
	"idios/clients/tui"
)

// ErrNonInteractive is returned when a full-screen launch is attempted without a terminal.
var ErrNonInteractive = errors.New("IDIOS TUI needs an interactive terminal; use `idios --help` for scriptable CLI/JSON commands")

// Run starts the standard TUI (demo=false) or the isolated sample-workspace TUI (demo=true).
// Both variants share the same interface and Engine; only their default database lifecycle differs.
func Run(demo bool) error {
	mode := "idios-tui"
	var defaultDB string
	if demo {
		mode = "idios-tui-demo"
		defaultDB = config.DefaultDemoDBPath()
	} else {
		defaultDB = config.DefaultDBPath()
	}

	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dbPath := flags.String("db", defaultDB, "Engine SQLite database path (overrides the mode-specific default)")
	noColor := flags.Bool("no-color", false, "disable ANSI colors (also disabled by NO_COLOR or TERM=dumb)")
	checkOnly := flags.Bool("check", false, "initialize/check the selected database and exit without opening the interactive UI")
	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	ctx := context.Background()
	if *checkOnly {
		app, closer, err := openApplication(ctx, *dbPath, demo)
		if err != nil {
			return err
		}
		defer closer.Close()
		repositories, err := app.ListRepositories(ctx)
		if err != nil {
			return fmt.Errorf("verify database: %w", err)
		}
		fmt.Fprintf(os.Stdout, "READY mode=%s repositories=%d database=%s\n", mode, len(repositories), *dbPath)
		return nil
	}

	appConfig, err := config.LoadAppConfig()
	if err != nil {
		return fmt.Errorf("error loading IDIOS configuration: %w (check ~/.idios/config.json or set IDIOS_CONFIG)", err)
	}
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
		return ErrNonInteractive
	}

	app, closer, err := openApplication(ctx, *dbPath, demo)
	if err != nil {
		return err
	}
	defer closer.Close()

	plain := *noColor || os.Getenv("NO_COLOR") != "" || strings.EqualFold(os.Getenv("TERM"), "dumb")
	return tui.RunInteractive(ctx, app, *dbPath, tui.Options{NoColor: plain, DemoMode: demo, Config: appConfig})
}

func openApplication(ctx context.Context, dbPath string, demo bool) (*application.Service, io.Closer, error) {
	store, err := sqlite.OpenStore(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error opening IDIOS database: %w", err)
	}
	app := application.New(store)
	if demo {
		if _, err := app.SeedDemo(ctx); err != nil {
			_ = store.Close()
			return nil, nil, fmt.Errorf("initialize demo workspace: %w", err)
		}
	}
	return app, store, nil
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
