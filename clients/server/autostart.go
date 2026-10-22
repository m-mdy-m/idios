package server

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

const serviceName = "idios.service"
const launchAgentLabel = "dev.idios.server"

// InstallAutostart registers a per-user startup entry. It never requests administrator access.
func InstallAutostart(binary string) error {
	absolute, err := filepath.Abs(binary)
	if err != nil {
		return fmt.Errorf("resolve IDIOS executable path: %w", err)
	}
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("systemctl"); err != nil {
			return fmt.Errorf("per-user autostart requires systemd and systemctl on Linux")
		}
		path := filepath.Join(userConfigHome(), "systemd", "user", serviceName)
		content := "[Unit]\nDescription=IDIOS local workspace server\nAfter=default.target\n\n[Service]\nType=simple\nExecStart=\"" + systemdQuote(absolute) + "\" serve\nRestart=on-failure\nRestartSec=3\n\n[Install]\nWantedBy=default.target\n"
		if err := writeFile(path, content, 0o600); err != nil {
			return err
		}
		if runErr := exec.Command("systemctl", "--user", "daemon-reload").Run(); runErr != nil {
			return fmt.Errorf("service file was installed but systemd user daemon-reload failed: %w", runErr)
		}
		if runErr := exec.Command("systemctl", "--user", "enable", "--now", serviceName).Run(); runErr != nil {
			return fmt.Errorf("service file was installed but could not be enabled: %w", runErr)
		}
		return nil
	case "darwin":
		if _, err := exec.LookPath("launchctl"); err != nil {
			return fmt.Errorf("per-user autostart requires launchctl on macOS")
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home directory: %w", err)
		}
		logDir := filepath.Join(home, "Library", "Logs", "IDIOS")
		if err := os.MkdirAll(logDir, 0o700); err != nil {
			return fmt.Errorf("create IDIOS log directory: %w", err)
		}
		path := filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
		if err := writeFile(path, launchAgentPlist(absolute, filepath.Join(logDir, "server.log")), 0o600); err != nil {
			return err
		}
		uid := "gui/"
		if current, userErr := user.Current(); userErr == nil {
			uid += current.Uid
		}
		if runErr := exec.Command("launchctl", "bootstrap", uid, path).Run(); runErr != nil {
			// An already-loaded LaunchAgent should not make installation fail.
			if !strings.Contains(strings.ToLower(runErr.Error()), "already loaded") {
				return fmt.Errorf("LaunchAgent was written but launchctl bootstrap failed: %w", runErr)
			}
		}
		return nil
	case "windows":
		scriptPath := filepath.Join(localAppDataDir(), "IDIOS", "start-server.vbs")
		command := `"` + absolute + `" serve`
		script := "Set shell = CreateObject(\"WScript.Shell\")\r\nshell.Run " + vbScriptString(command) + ", 0, False\r\n"
		if err := writeFile(scriptPath, script, 0o600); err != nil {
			return err
		}
		value := `wscript.exe "` + scriptPath + `"`
		cmd := exec.Command("reg.exe", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "IDIOS", "/t", "REG_SZ", "/d", value, "/f")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("register IDIOS startup entry: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	default:
		return fmt.Errorf("automatic startup is not supported on %s", runtime.GOOS)
	}
}

// UninstallAutostart removes IDIOS's per-user startup entry without touching user data.
func UninstallAutostart() error {
	switch runtime.GOOS {
	case "linux":
		path := filepath.Join(userConfigHome(), "systemd", "user", serviceName)
		if _, err := exec.LookPath("systemctl"); err == nil {
			_ = exec.Command("systemctl", "--user", "disable", "--now", serviceName).Run()
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove IDIOS service file: %w", err)
		}
		if _, err := exec.LookPath("systemctl"); err == nil {
			_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		}
		return nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path := filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
		uid := "gui/"
		if current, userErr := user.Current(); userErr == nil {
			uid += current.Uid
		}
		_ = exec.Command("launchctl", "bootout", uid, path).Run()
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove IDIOS LaunchAgent: %w", err)
		}
		return nil
	case "windows":
		cmd := exec.Command("reg.exe", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "IDIOS", "/f")
		if output, err := cmd.CombinedOutput(); err != nil && !bytes.Contains(bytes.ToLower(output), []byte("unable to find")) {
			return fmt.Errorf("remove IDIOS startup entry: %w: %s", err, strings.TrimSpace(string(output)))
		}
		scriptPath := filepath.Join(localAppDataDir(), "IDIOS", "start-server.vbs")
		if err := os.Remove(scriptPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove IDIOS startup script: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("automatic startup is not supported on %s", runtime.GOOS)
	}
}

// AutostartInstalled reports whether a per-user startup entry exists.
func AutostartInstalled() bool {
	switch runtime.GOOS {
	case "linux":
		_, err := os.Stat(filepath.Join(userConfigHome(), "systemd", "user", serviceName))
		return err == nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		_, err = os.Stat(filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist"))
		return err == nil
	case "windows":
		return exec.Command("reg.exe", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "IDIOS").Run() == nil
	default:
		return false
	}
}

// StartDetached starts the current executable without tying its lifetime to this CLI process.
func StartDetached(binary string, args ...string) error {
	cmd := exec.Command(binary, args...)
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open null device: %w", err)
	}
	defer devNull.Close()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devNull, devNull, devNull
	configureDetached(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start IDIOS server: %w", err)
	}
	return cmd.Process.Release()
}

func userConfigHome() string {
	if value := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config")
	}
	return filepath.Join(home, ".config")
}

func writeFile(path, content string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create startup directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".idios-startup-*.tmp")
	if err != nil {
		return fmt.Errorf("create startup file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("write startup file %q: %w", path, err)
	}
	return nil
}

func systemdQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func launchAgentPlist(binary, logPath string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>` + xmlEscape(launchAgentLabel) + `</string>
<key>ProgramArguments</key><array><string>` + xmlEscape(binary) + `</string><string>serve</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
<key>StandardOutPath</key><string>` + xmlEscape(logPath) + `</string>
<key>StandardErrorPath</key><string>` + xmlEscape(logPath) + `</string>
</dict></plist>
`
}

func xmlEscape(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	return strings.ReplaceAll(value, "'", "&apos;")
}

// localAppDataDir resolves the current user's private local application-data directory.
func localAppDataDir() string {
	if value := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "AppData", "Local")
	}
	return filepath.Join(home, "AppData", "Local")
}

func vbScriptString(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
