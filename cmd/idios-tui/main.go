package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"idios/internal/application"
	"idios/internal/config"
	"idios/internal/storage/sqlite"
	"idios/internal/tui"
)

func main() {
	dbPath := flag.String("db", config.DefaultDBPath(), "Engine SQLite database path")
	noColor := flag.Bool("no-color", false, "disable ANSI colors (also disabled by NO_COLOR or TERM=dumb)")
	flag.Parse()

	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
		fmt.Fprintln(os.Stderr, "idios-tui needs an interactive terminal; use `idios --help` for scriptable CLI/JSON commands")
		os.Exit(2)
	}

	store, err := sqlite.OpenStore(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error opening IDIOS database:", err)
		os.Exit(1)
	}
	defer store.Close()

	plain := *noColor || os.Getenv("NO_COLOR") != "" || strings.EqualFold(os.Getenv("TERM"), "dumb")
	if err := tui.RunInteractive(context.Background(), application.New(store), *dbPath, tui.Options{NoColor: plain}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
