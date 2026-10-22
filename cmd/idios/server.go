package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"idios/internal/application"
	"idios/clients/cli"
	"idios/internal/config"
	"idios/clients/server"
	"idios/internal/storage/sqlite"
)

func isServerCommand(command string) bool {
	switch command {
	case "serve", "web", "server":
		return true
	default:
		return false
	}
}

func runServerCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printServerHelp(stdout)
		return 0
	}
	switch args[0] {
	case "serve":
		// Accept the natural "idios serve install" spelling as an alias for
		// "idios server install" (and likewise for other control actions).
		if len(args) > 1 && isServerControlAction(args[1]) {
			return runServerControl(args[1:], stdout, stderr)
		}
		return runServe(args[1:], stdout, stderr)
	case "web":
		return runWeb(args[1:], stdout, stderr)
	case "server":
		return runServerControl(args[1:], stdout, stderr)
	default:
		printServerHelp(stderr)
		return 2
	}
}

func isServerControlAction(action string) bool {
	switch action {
	case "start", "stop", "status", "install", "uninstall":
		return true
	default:
		return false
	}
}

func runServe(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("idios serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dbPath := flags.String("db", config.DefaultDBPath(), "Engine SQLite database path")
	addr := flags.String("addr", "", "loopback address override (host:port)")
	openBrowser := flags.Bool("open", false, "open the local web interface after the server starts")
	noOpenBrowser := flags.Bool("no-open", false, "do not open a browser even when enabled in config")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}
	cfg, err := config.LoadAppConfig()
	if err != nil {
		fmt.Fprintf(stderr, "load IDIOS config: %v\n", err)
		return 1
	}
	baseURL, err := server.BaseURL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "resolve IDIOS address: %v\n", err)
		return 1
	}
	if *addr != "" {
		baseURL = "http://" + strings.TrimSpace(*addr)
	}
	if *openBrowser {
		cfg.Server.OpenBrowser = true
	}
	if *noOpenBrowser {
		cfg.Server.OpenBrowser = false
	}
	store, err := sqlite.OpenStore(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "open IDIOS database: %v\n", err)
		return 1
	}
	app := application.New(store)
	defer func() { _ = app.Close() }()
	options := server.Options{
		Config: cfg, ConfigPath: config.AppConfigPath(), DBPath: *dbPath, Version: cli.Version,
		Address: strings.TrimPrefix(baseURL, "http://"),
		OnStart: func(address string) {
			fmt.Fprintf(stdout, "IDIOS server listening at %s\n", address)
			if cfg.Server.OpenBrowser {
				if err := server.OpenBrowser(address); err != nil {
					fmt.Fprintf(stderr, "open browser: %v\n", err)
				}
			}
		},
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := server.New(app, options).Run(ctx); err != nil {
		fmt.Fprintf(stderr, "IDIOS server: %v\n", err)
		return 1
	}
	return 0
}

func runWeb(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("idios web", flag.ContinueOnError)
	flags.SetOutput(stderr)
	noOpen := flags.Bool("no-open", false, "print the URL without opening a browser")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}
	cfg, err := config.LoadAppConfig()
	if err != nil {
		fmt.Fprintf(stderr, "load IDIOS config: %v\n", err)
		return 1
	}
	baseURL, err := server.ControlURL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "resolve IDIOS address: %v\n", err)
		return 1
	}
	if running, _ := server.Probe(baseURL); !running {
		executable, exeErr := os.Executable()
		if exeErr != nil {
			fmt.Fprintf(stderr, "resolve IDIOS executable: %v\n", exeErr)
			return 1
		}
		if err := server.StartDetached(executable, "serve", "--no-open"); err != nil {
			fmt.Fprintf(stderr, "start IDIOS server: %v\n", err)
			return 1
		}
		if !waitForServer(baseURL, 5*time.Second) {
			fmt.Fprintf(stderr, "IDIOS server did not become ready at %s\n", baseURL)
			return 1
		}
	}
	fmt.Fprintln(stdout, baseURL)
	if !*noOpen {
		if err := server.OpenBrowser(baseURL); err != nil {
			fmt.Fprintf(stderr, "open browser: %v\n", err)
			return 1
		}
	}
	return 0
}

func runServerControl(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printServerHelp(stdout)
		return 0
	}
	cfg, err := config.LoadAppConfig()
	if err != nil {
		fmt.Fprintf(stderr, "load IDIOS config: %v\n", err)
		return 1
	}
	baseURL, err := server.ControlURL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "resolve IDIOS address: %v\n", err)
		return 1
	}
	switch args[0] {
	case "status":
		running, payload := server.Probe(baseURL)
		if running {
			fmt.Fprintf(stdout, "IDIOS server: running\nURL: %s\n", baseURL)
			if version, ok := payload["version"].(string); ok {
				fmt.Fprintf(stdout, "Version: %s\n", version)
			}
		} else {
			fmt.Fprintf(stdout, "IDIOS server: stopped\nURL: %s\n", baseURL)
		}
		if server.AutostartInstalled() {
			fmt.Fprintln(stdout, "Autostart: installed")
		} else {
			fmt.Fprintln(stdout, "Autostart: not installed")
		}
		if running {
			return 0
		}
		return 1
	case "start":
		if running, _ := server.Probe(baseURL); running {
			fmt.Fprintf(stdout, "IDIOS server is already running at %s\n", baseURL)
			return 0
		}
		executable, exeErr := os.Executable()
		if exeErr != nil {
			fmt.Fprintf(stderr, "resolve IDIOS executable: %v\n", exeErr)
			return 1
		}
		if err := server.StartDetached(executable, "serve"); err != nil {
			fmt.Fprintf(stderr, "start IDIOS server: %v\n", err)
			return 1
		}
		if !waitForServer(baseURL, 5*time.Second) {
			fmt.Fprintf(stderr, "IDIOS server did not become ready at %s\n", baseURL)
			return 1
		}
		fmt.Fprintf(stdout, "IDIOS server started at %s\n", baseURL)
		return 0
	case "stop":
		if running, _ := server.Probe(baseURL); !running {
			fmt.Fprintln(stdout, "IDIOS server is not running")
			return 0
		}
		if err := server.RequestShutdown(baseURL); err != nil {
			fmt.Fprintf(stderr, "stop IDIOS server: %v\n", err)
			return 1
		}
		if waitForServerStop(baseURL, 5*time.Second) {
			fmt.Fprintln(stdout, "IDIOS server stopped")
			return 0
		}
		fmt.Fprintln(stderr, "IDIOS server accepted the stop request but is still responding")
		return 1
	case "install":
		executable, exeErr := os.Executable()
		if exeErr != nil {
			fmt.Fprintf(stderr, "resolve IDIOS executable: %v\n", exeErr)
			return 1
		}
		if err := server.InstallAutostart(executable); err != nil {
			fmt.Fprintf(stderr, "install IDIOS autostart: %v\n", err)
			return 1
		}
		if running, _ := server.Probe(baseURL); !running {
			if err := server.StartDetached(executable, "serve"); err != nil {
				fmt.Fprintf(stderr, "startup entry installed but server could not be started: %v\n", err)
				return 1
			}
			if !waitForServer(baseURL, 5*time.Second) {
				fmt.Fprintf(stderr, "startup entry installed, but server is not responding at %s\n", baseURL)
				return 1
			}
		}
		fmt.Fprintln(stdout, "IDIOS autostart installed and server is running for this user.")
		return 0
	case "uninstall":
		if running, _ := server.Probe(baseURL); running {
			if err := server.RequestShutdown(baseURL); err == nil {
				_ = waitForServerStop(baseURL, 5*time.Second)
			}
		}
		if err := server.UninstallAutostart(); err != nil {
			fmt.Fprintf(stderr, "uninstall IDIOS autostart: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "IDIOS autostart removed. Database and config were kept.")
		return 0
	default:
		printServerHelp(stderr)
		return 2
	}
}

func waitForServer(baseURL string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if running, _ := server.Probe(baseURL); running {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func waitForServerStop(baseURL string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if running, _ := server.Probe(baseURL); !running {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func printServerHelp(w io.Writer) {
	fmt.Fprintln(w, `IDIOS local web server

Usage:
  idios serve [--addr 127.0.0.1:43170] [--db PATH] [--open] [--no-open]
  idios web [--no-open]
  idios server start|stop|status|install|uninstall
  idios serve start|stop|status|install|uninstall  (aliases)

The server binds to loopback only. 'install' enables per-user startup on login.`)
}
