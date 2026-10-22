package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"idios/internal/application"
	appconfig "idios/internal/config"
	"idios/internal/logger"
	"idios/clients/tui/render"
)

// Version is set to the source version by default and overridden by release builds via -ldflags.
var Version = "1.0.2"

// config holds the global flags, which may appear anywhere in the argument list.
type config struct {
	dbPath   string
	json     bool
	logLevel string
}

// DefaultDBPath remains exported for callers that previously obtained the shared default here.
func DefaultDBPath() string { return appconfig.DefaultDBPath() }

// logger enables monitoring only when asked for, keeping stdout/stderr contract-clean.
func (c config) logger(w io.Writer) *logger.Logger {
	if c.logLevel == "" {
		return logger.FromEnv(w)
	}
	return logger.New(w, c.logLevel)
}

type AppFactory func(dbPath string) (*application.Service, io.Closer, error)

var errTUIWanted = errors.New("use the separate `idios-tui` executable for the TUI")

// globals strips the global flags out of args wherever they appear.
func globals(args []string) (config, []string, error) {
	c := config{dbPath: DefaultDBPath()}
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
			c.json = true
		case a == "--no-json":
			c.json = false
		case a == "--verbose":
			c.logLevel = "debug"
		case a == "--log-level":
			if i+1 >= len(args) {
				return c, nil, errors.New("--log-level needs a value")
			}
			i++
			c.logLevel = args[i]
		case strings.HasPrefix(a, "--log-level="):
			c.logLevel = strings.TrimPrefix(a, "--log-level=")
		case a == "--db":
			if i+1 >= len(args) {
				return c, nil, errors.New("--db needs a path")
			}
			i++
			c.dbPath = args[i]
		case strings.HasPrefix(a, "--db="):
			c.dbPath = strings.TrimPrefix(a, "--db=")
		default:
			out = append(out, a)
		}
	}
	if c.dbPath == "" {
		return c, nil, errors.New("database path cannot be empty")
	}
	return c, out, nil
}

// Run parses the CLI contract and dispatches commands. The composition root supplies an
// Application factory, keeping this adapter independent of concrete storage.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, factory AppFactory) int {
	cfg, args, err := globals(args)
	log := cfg.logger(stderr)
	if err != nil {
		report(stderr, cfg.json, args, err, log)
		return 2
	}
	if len(args) == 0 {
		printHelp(stdout)
		return 0
	}
	switch args[0] {
	case "help", "--help", "-h":
		return runHelp(stdout, stderr, cfg.json, args[1:])
	case "version", "--version":
		fmt.Fprintln(stdout, Version)
		return 0
	case "protocol":
		render.Value(stdout, map[string]any{
			"protocol_version":       1,
			"application_version":    Version,
			"json_flag":              "--json",
			"issue_reference":        "Repository#number",
			"pull_request_reference": "Repository!number",
		}, cfg.json)
		return 0
	case "tui":
		report(stderr, cfg.json, args, errTUIWanted, log)
		return 2
	}
	if factory == nil {
		report(stderr, cfg.json, args, errors.New("Engine application factory is not configured"), log)
		return 1
	}
	svc, closer, err := factory(cfg.dbPath)
	if err != nil {
		report(stderr, cfg.json, args, fmt.Errorf("opening IDIOS Engine: %w", err), log)
		return 1
	}
	if closer != nil {
		defer func() { _ = closer.Close() }()
	}
	// Flag-parse chatter stays out of machine-readable output.
	flagOut := stderr
	if cfg.json {
		flagOut = io.Discard
	}
	started := time.Now()
	log.Debug("command", "command", strings.Join(args, " "), "database", cfg.dbPath)
	if cmd, _ := resolve(args); cmd == nil && hasGroup(args[0]) && hasHelpFlag(args) {
		return runHelp(stdout, stderr, cfg.json, args)
	}
	value, err := dispatch(ctx, svc, args, flagOut)
	if err != nil {
		if isHelpRequest(err) {
			writeHelp(stdout, stderr, cfg.json, args)
			return 0
		}
		report(stderr, cfg.json, args, err, log)
		return 1
	}
	log.Debug("command done", "command", strings.Join(args, " "), "duration", time.Since(started).String())
	if value != nil {
		render.Value(stdout, value, cfg.json)
	}
	return 0
}

// runHelp answers `idios help [COMMAND...]` and `idios <group> --help`.
func runHelp(stdout, stderr io.Writer, asJSON bool, args []string) int {
	if len(args) == 0 {
		printHelp(stdout)
		return 0
	}
	if cmd, _ := resolve(args); cmd != nil {
		printCommandHelp(stdout, cmd)
		return 0
	}
	// A group name is not a runnable command, but it is a valid help topic.
	if word := args[0]; hasGroup(word) {
		printGroupHelp(stdout, word)
		return 0
	}
	fmt.Fprintf(stderr, "unknown command %q\n\n", strings.Join(args, " "))
	printHelp(stderr)
	return 1
}

// hasHelpFlag reports a `--help`/`-h` anywhere after the command words.
func hasHelpFlag(args []string) bool {
	for _, a := range args[1:] {
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

// writeHelp prints help for the command named in args, keeping stdout clean in JSON mode.
func writeHelp(stdout, stderr io.Writer, asJSON bool, args []string) {
	out := stdout
	if asJSON {
		out = stderr
	}
	if cmd, _ := resolve(args); cmd != nil {
		printCommandHelp(out, cmd)
		return
	}
	printHelp(out)
}
