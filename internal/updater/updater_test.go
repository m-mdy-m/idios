package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsNewerSemanticVersions(t *testing.T) {
	tests := []struct {
		candidate string
		current   string
		want      bool
		wantErr   bool
	}{
		{"1.0.1", "1.0.0", true, false},
		{"v1.10.0", "1.9.9", true, false},
		{"1.2.0", "1.2.0", false, false},
		{"1.2.0-rc.2", "1.2.0-rc.1", true, false},
		{"1.2.0", "1.2.0-rc.9", true, false},
		{"1.2.0-rc.1", "1.2.0", false, false},
		{"1.0", "1.0.0", false, true},
		{"not-a-version", "dev", false, true},
		{"1.0.1", "dev", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.candidate+"_vs_"+tt.current, func(t *testing.T) {
			got, err := IsNewer(tt.candidate, tt.current)
			if (err != nil) != tt.wantErr {
				t.Fatalf("IsNewer() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("IsNewer(%q, %q) = %v, want %v", tt.candidate, tt.current, got, tt.want)
			}
		})
	}
}

func TestSectionForVersion(t *testing.T) {
	changelog := "# Changelog\n\n## [1.2.0] — Unreleased\n\n### Added\n- New release notes.\n- More details.\n\n## [1.1.0]\n- Old release.\n"
	got := sectionForVersion(changelog, "1.2.0")
	if !strings.Contains(got, "- New release notes.") || strings.Contains(got, "Old release") {
		t.Fatalf("sectionForVersion() = %q", got)
	}
	if got := sectionForVersion(changelog, "9.9.9"); got != "" {
		t.Fatalf("sectionForVersion() for missing version = %q, want empty", got)
	}
}

func TestChecksumFor(t *testing.T) {
	const want = "8f434346648f6b96df89dda901c5176b10a6d83961dd3c1ac88b59b2dc327aa4"
	got, err := checksumFor(fmt.Sprintf("%s  idios-v1.2.3-linux-amd64.tar.gz\n", want), "idios-v1.2.3-linux-amd64.tar.gz")
	if err != nil || got != want {
		t.Fatalf("checksumFor() = %q, %v; want %q", got, err, want)
	}
	if _, err := checksumFor("abc  another.zip\n", "idios.zip"); err == nil {
		t.Fatal("checksumFor() accepted a missing asset")
	}
}

func TestExtractExecutablesFromTarGz(t *testing.T) {
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tw := tar.NewWriter(gz)
	for _, entry := range []struct{ name, body string }{{"./idios", "cli-binary"}, {"./idios-tui", "tui-binary"}, {"README.md", "not executable"}} {
		if err := tw.WriteHeader(&tar.Header{Name: entry.name, Mode: 0o755, Size: int64(len(entry.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	files, err := extractExecutables(raw.Bytes(), "idios-v1.2.3-linux-amd64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if string(files["idios"]) != "cli-binary" || string(files["idios-tui"]) != "tui-binary" {
		t.Fatalf("unexpected extracted payloads: %#v", files)
	}
}

func TestExtractExecutablesFromZip(t *testing.T) {
	var raw bytes.Buffer
	zw := zip.NewWriter(&raw)
	for name, body := range map[string]string{"idios.exe": "cli-binary", "idios-tui.exe": "tui-binary"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	files, err := extractExecutables(raw.Bytes(), "idios-v1.2.3-windows-amd64.zip")
	if err != nil {
		t.Fatal(err)
	}
	if string(files["idios"]) != "cli-binary" || string(files["idios-tui"]) != "tui-binary" {
		t.Fatalf("unexpected extracted payloads: %#v", files)
	}
}

func TestReleaseAssetNames(t *testing.T) {
	if got := releaseAssetName("v1.2.3", "linux", "arm64"); got != "idios-v1.2.3-linux-arm64.tar.gz" {
		t.Fatalf("Linux asset name = %q", got)
	}
	if got := releaseAssetName("v1.2.3", "windows", "amd64"); got != "idios-v1.2.3-windows-amd64.zip" {
		t.Fatalf("Windows asset name = %q", got)
	}
}

func TestUnixHelperHasValidShellSyntax(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell syntax is not available on native Windows runners")
	}
	dir := filepath.Join(t.TempDir(), "dir with 'quote")
	prepared := Prepared{
		StagedCLI: filepath.Join(dir, ".idios.new"),
		StagedTUI: filepath.Join(dir, ".idios-tui.new"),
		DestCLI:   filepath.Join(dir, "idios"),
		DestTUI:   filepath.Join(dir, "idios-tui"),
	}
	scriptPath := filepath.Join(t.TempDir(), "upgrade-helper.sh")
	if err := os.WriteFile(scriptPath, []byte(unixHelper(prepared, 42)), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("sh", "-n", scriptPath).CombinedOutput(); err != nil {
		t.Fatalf("generated Unix helper has invalid shell syntax: %v\n%s", err, output)
	}
}

func TestWindowsHelperStartsNewAppBeforeRemovingBackups(t *testing.T) {
	prepared := Prepared{
		StagedCLI: `C:\Program Files\IDIOS\.idios.new.exe`,
		StagedTUI: `C:\Program Files\IDIOS\.idios-tui.new.exe`,
		DestCLI:   `C:\Program Files\IDIOS\idios.exe`,
		DestTUI:   `C:\Program Files\IDIOS\idios-tui.exe`,
	}
	script := windowsHelper(prepared, 42)
	start := strings.Index(script, "Start-Process -FilePath $destCli")
	cleanup := strings.Index(script, "Remove-Item -LiteralPath $backupCli")
	if start < 0 || cleanup < 0 || cleanup < start {
		t.Fatalf("Windows helper must start the new executable before deleting backups:\n%s", script)
	}
}
