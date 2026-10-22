//go:build windows

package server

import (
	"fmt"
	"os/exec"
)

func openBrowser(url string) error {
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return cmd.Process.Release()
}
