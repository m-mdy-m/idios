//go:build !windows

package server

import (
	"fmt"
	"os/exec"
	"runtime"
)

func openBrowser(url string) error {
	var command string
	switch runtime.GOOS {
	case "darwin":
		command = "open"
	case "linux", "freebsd", "openbsd", "netbsd":
		command = "xdg-open"
	default:
		return fmt.Errorf("opening a browser is not supported on %s", runtime.GOOS)
	}
	cmd := exec.Command(command, url)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return cmd.Process.Release()
}
