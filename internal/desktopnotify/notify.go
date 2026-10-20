// Package desktopnotify delivers a short native desktop notification where supported.
// IDIOS still records the message in its own inbox if the operating-system delivery fails.
package desktopnotify

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
)

// Send attempts a platform-native notification. It never invokes a shell on Unix-like systems.
func Send(ctx context.Context, title, body string) error {
	title = compact(title, 100)
	body = compact(body, 400)
	if title == "" {
		title = "IDIOS"
	}
	if body == "" {
		body = "You have a new quote."
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	switch runtime.GOOS {
	case "linux":
		if path, err := exec.LookPath("notify-send"); err == nil {
			return run(ctx, path, "--app-name=IDIOS", "--icon=dialog-information", title, body)
		}
		return fmt.Errorf("no supported Linux notification command found (install notify-send/libnotify)")
	case "darwin":
		path, err := exec.LookPath("osascript")
		if err != nil {
			return fmt.Errorf("osascript is unavailable: %w", err)
		}
		script := `display notification "` + escapeAppleScript(body) + `" with title "` + escapeAppleScript(title) + `"`
		return run(ctx, path, "-e", script)
	case "windows":
		return sendWindows(ctx, title, body)
	default:
		return fmt.Errorf("desktop notifications are not implemented for %s", runtime.GOOS)
	}
}

func run(ctx context.Context, path string, args ...string) error {
	cmd := exec.CommandContext(ctx, path, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return fmt.Errorf("%s failed: %w", path, err)
		}
		return fmt.Errorf("%s failed: %w: %s", path, err, compact(message, 240))
	}
	return nil
}

func sendWindows(ctx context.Context, title, body string) error {
	path, err := exec.LookPath("powershell.exe")
	if err != nil {
		path, err = exec.LookPath("powershell")
		if err != nil {
			return fmt.Errorf("PowerShell is unavailable for Windows desktop notification: %w", err)
		}
	}
	// NotifyIcon balloon delivery works for unpackaged console applications without requiring
	// a separately installed PowerShell module or registering an AppUserModelID.
	script := "$ErrorActionPreference='Stop'; Add-Type -AssemblyName System.Windows.Forms; Add-Type -AssemblyName System.Drawing; " +
		"$n=New-Object System.Windows.Forms.NotifyIcon; $n.Icon=[System.Drawing.SystemIcons]::Information; " +
		"$n.BalloonTipTitle='" + escapePowerShell(title) + "'; $n.BalloonTipText='" + escapePowerShell(body) + "'; " +
		"$n.Visible=$true; $n.ShowBalloonTip(6000); Start-Sleep -Milliseconds 6500; $n.Visible=$false; $n.Dispose()"
	encoded := base64.StdEncoding.EncodeToString(utf16LE(script))
	return run(ctx, path, "-NoProfile", "-NonInteractive", "-STA", "-EncodedCommand", encoded)
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

func escapePowerShell(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func utf16LE(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, len(units)*2)
	for i, unit := range units {
		out[i*2] = byte(unit)
		out[i*2+1] = byte(unit >> 8)
	}
	return out
}

func compact(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes-1]) + "…"
	}
	return s
}
