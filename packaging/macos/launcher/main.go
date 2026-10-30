//go:build darwin

package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	if err := launch(); err != nil {
		logFailure(err)
		os.Exit(1)
	}
}

func launch() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	bundle := filepath.Clean(filepath.Join(filepath.Dir(executable), "..", ".."))
	server := filepath.Join(bundle, "Contents", "Resources", "idios")
	if info, err := os.Stat(server); err != nil || info.IsDir() {
		return os.ErrNotExist
	}

	logFile := logPath()
	if err := os.MkdirAll(filepath.Dir(logFile), 0o700); err != nil {
		return err
	}
	output, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer output.Close()

	stdin, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	defer stdin.Close()

	command := exec.Command(server, "serve", "--open")
	command.Stdin = stdin
	command.Stdout = output
	command.Stderr = output
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func logPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "idios-desktop-launcher.log")
	}
	return filepath.Join(home, "Library", "Logs", "IDIOS", "desktop-launcher.log")
}

func logFailure(err error) {
	path := logPath()
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	file, openErr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if openErr != nil {
		return
	}
	defer file.Close()
	log.New(file, "IDIOS launcher: ", log.LstdFlags).Print(err)
}
