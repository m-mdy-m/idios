// Package updater checks published GitHub releases and prepares a verified in-place update.
// Updates are staged next to the installed binaries so the final replacement stays on the
// same filesystem. A tiny OS-native helper waits for IDIOS to exit before swapping files.
package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	Owner      = "m-mdy-m"
	Repository = "idios"
	apiLatest  = "https://api.github.com/repos/" + Owner + "/" + Repository + "/releases/latest"
	maxArchive = 180 << 20
)

// Version is overridden in release builds with -ldflags for every executable.
var Version = "2.0.0"

type Release struct {
	Version      string
	Tag          string
	URL          string
	Notes        string
	AssetName    string
	AssetURL     string
	ChecksumsURL string
}

type releaseResponse struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func Check(ctx context.Context, currentVersion string) (*Release, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiLatest, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "IDIOS-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check GitHub Releases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("check GitHub Releases: HTTP %s", resp.Status)
	}
	var payload releaseResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("read GitHub release metadata: %w", err)
	}
	latestVersion := strings.TrimPrefix(strings.TrimSpace(payload.TagName), "v")
	if latestVersion == "" || payload.TagName == "" {
		return nil, errors.New("GitHub returned a release without a version tag")
	}
	newer, err := IsNewer(latestVersion, currentVersion)
	if err != nil {
		return nil, err
	}
	if !newer {
		return nil, nil
	}

	assetName := releaseAssetName(payload.TagName, runtime.GOOS, runtime.GOARCH)
	var assetURL, checksumsURL string
	for _, a := range payload.Assets {
		switch a.Name {
		case assetName:
			assetURL = a.BrowserDownloadURL
		case "SHA256SUMS.txt":
			checksumsURL = a.BrowserDownloadURL
		}
	}
	if assetURL == "" || checksumsURL == "" {
		return nil, fmt.Errorf("release %s is missing the platform bundle or SHA256SUMS.txt", payload.TagName)
	}

	release := &Release{
		Version: latestVersion, Tag: payload.TagName, URL: payload.HTMLURL,
		Notes: strings.TrimSpace(payload.Body), AssetName: assetName,
		AssetURL: assetURL, ChecksumsURL: checksumsURL,
	}
	if notes := fetchChangelog(ctx, payload.TagName, latestVersion); notes != "" {
		release.Notes = notes
	}
	if release.Notes == "" {
		release.Notes = "This release does not include published release notes. Open the release page for details."
	}
	return release, nil
}

func releaseAssetName(tag, goos, goarch string) string {
	asset := goos + "-" + goarch
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return "idios-" + tag + "-" + asset + ext
}

var changelogHeading = regexp.MustCompile(`^## \[v?([^\r\n]+)\]`)

func fetchChangelog(ctx context.Context, tag, version string) string {
	url := "https://raw.githubusercontent.com/" + Owner + "/" + Repository + "/" + tag + "/CHANGELOG.md"
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "IDIOS-Updater")
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	contents, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}
	return sectionForVersion(string(contents), version)
}

func sectionForVersion(changelog, version string) string {
	lines := strings.Split(strings.ReplaceAll(changelog, "\r\n", "\n"), "\n")
	start := -1
	for i, line := range lines {
		matches := changelogHeading.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) == 2 && strings.TrimPrefix(matches[1], "v") == version {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

// IsNewer returns whether candidate is newer than current, comparing numeric semantic-version
// components and prerelease identifiers. The inputs may include a leading v and build metadata.
func IsNewer(candidate, current string) (bool, error) {
	c, err := parseVersion(candidate)
	if err != nil {
		return false, fmt.Errorf("invalid latest version %q: %w", candidate, err)
	}
	v, err := parseVersion(current)
	if err != nil {
		// Development builds are treated as older than any published semantic release.
		if strings.EqualFold(strings.TrimSpace(current), "dev") || strings.TrimSpace(current) == "" {
			return true, nil
		}
		return false, fmt.Errorf("invalid current version %q: %w", current, err)
	}
	return compareVersion(c, v) > 0, nil
}

type semanticVersion struct {
	major, minor, patch uint64
	prerelease          []string
}

func parseVersion(s string) (semanticVersion, error) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "v"))
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	parts := strings.SplitN(s, "-", 2)
	core := strings.Split(parts[0], ".")
	if len(core) != 3 {
		return semanticVersion{}, errors.New("expected major.minor.patch")
	}
	var v semanticVersion
	numbers := []*uint64{&v.major, &v.minor, &v.patch}
	for i, part := range core {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return semanticVersion{}, errors.New("invalid numeric component")
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return semanticVersion{}, errors.New("invalid numeric component")
			}
		}
		var n uint64
		for _, r := range part {
			if n > (^uint64(0)-uint64(r-'0'))/10 {
				return semanticVersion{}, errors.New("numeric component overflows")
			}
			n = n*10 + uint64(r-'0')
		}
		*numbers[i] = n
	}
	if len(parts) == 2 {
		if parts[1] == "" {
			return semanticVersion{}, errors.New("empty prerelease identifier")
		}
		v.prerelease = strings.Split(parts[1], ".")
		for _, id := range v.prerelease {
			if id == "" {
				return semanticVersion{}, errors.New("empty prerelease identifier")
			}
			for _, r := range id {
				if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
					return semanticVersion{}, errors.New("invalid prerelease identifier")
				}
			}
		}
	}
	return v, nil
}

func compareVersion(a, b semanticVersion) int {
	for _, pair := range [][2]uint64{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if len(a.prerelease) == 0 && len(b.prerelease) > 0 {
		return 1
	}
	if len(a.prerelease) > 0 && len(b.prerelease) == 0 {
		return -1
	}
	for i := 0; i < len(a.prerelease) && i < len(b.prerelease); i++ {
		x, y := a.prerelease[i], b.prerelease[i]
		if x == y {
			continue
		}
		xNum, xErr := numericIdentifier(x)
		yNum, yErr := numericIdentifier(y)
		switch {
		case xErr == nil && yErr == nil:
			if xNum < yNum {
				return -1
			}
			return 1
		case xErr == nil:
			return -1
		case yErr == nil:
			return 1
		case x < y:
			return -1
		default:
			return 1
		}
	}
	if len(a.prerelease) < len(b.prerelease) {
		return -1
	}
	if len(a.prerelease) > len(b.prerelease) {
		return 1
	}
	return 0
}

func numericIdentifier(s string) (uint64, error) {
	if s == "" {
		return 0, errors.New("empty")
	}
	var n uint64
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errors.New("not numeric")
		}
		if n > (^uint64(0)-uint64(r-'0'))/10 {
			return 0, errors.New("overflow")
		}
		n = n*10 + uint64(r-'0')
	}
	return n, nil
}

type Prepared struct {
	StagedCLI string
	StagedTUI string
	DestCLI   string
	DestTUI   string
	Launcher  string
}

func (p Prepared) Cleanup() {
	if p.StagedCLI != "" {
		_ = os.Remove(p.StagedCLI)
	}
	if p.StagedTUI != "" {
		_ = os.Remove(p.StagedTUI)
	}
}

func Prepare(ctx context.Context, release Release, executablePath string) (Prepared, error) {
	var prepared Prepared
	if release.AssetURL == "" || release.ChecksumsURL == "" || release.AssetName == "" {
		return prepared, errors.New("release download metadata is incomplete")
	}
	client := &http.Client{Timeout: 90 * time.Second}
	archiveBytes, err := fetchLimited(ctx, client, release.AssetURL, maxArchive)
	if err != nil {
		return prepared, fmt.Errorf("download release bundle: %w", err)
	}
	checksumBytes, err := fetchLimited(ctx, client, release.ChecksumsURL, 2<<20)
	if err != nil {
		return prepared, fmt.Errorf("download release checksums: %w", err)
	}
	expected, err := checksumFor(string(checksumBytes), release.AssetName)
	if err != nil {
		return prepared, err
	}
	actual := sha256.Sum256(archiveBytes)
	if !strings.EqualFold(expected, hex.EncodeToString(actual[:])) {
		return prepared, errors.New("release checksum mismatch: downloaded bundle was not modified by IDIOS release")
	}

	binaries, err := extractExecutables(archiveBytes, release.AssetName)
	if err != nil {
		return prepared, err
	}
	if len(binaries["idios"]) == 0 || len(binaries["idios-tui"]) == 0 {
		return prepared, errors.New("release bundle does not contain both idios and idios-tui")
	}

	executablePath, err = filepath.Abs(executablePath)
	if err != nil {
		return prepared, err
	}
	dir := filepath.Dir(executablePath)
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	prepared = Prepared{
		DestCLI:  filepath.Join(dir, "idios"+ext),
		DestTUI:  filepath.Join(dir, "idios-tui"+ext),
		Launcher: filepath.Join(dir, "idios"+ext),
	}
	prepared.StagedCLI, err = writeStaged(dir, ".idios-upgrade-cli-*", binaries["idios"])
	if err != nil {
		return Prepared{}, fmt.Errorf("stage the new idios executable: %w", err)
	}
	prepared.StagedTUI, err = writeStaged(dir, ".idios-upgrade-tui-*", binaries["idios-tui"])
	if err != nil {
		prepared.Cleanup()
		return Prepared{}, fmt.Errorf("stage the new idios-tui executable: %w", err)
	}
	return prepared, nil
}

func fetchLimited(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "IDIOS-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("download exceeds %d bytes", limit)
	}
	return b, nil
}

func checksumFor(contents, assetName string) (string, error) {
	for _, line := range strings.Split(strings.ReplaceAll(contents, "\r\n", "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && filepath.Base(strings.TrimPrefix(fields[len(fields)-1], "*")) == assetName {
			hash := strings.ToLower(fields[0])
			if len(hash) != 64 {
				return "", errors.New("invalid SHA-256 checksum entry for release bundle")
			}
			if _, err := hex.DecodeString(hash); err != nil {
				return "", errors.New("invalid SHA-256 checksum encoding")
			}
			return hash, nil
		}
	}
	return "", fmt.Errorf("SHA256SUMS.txt does not contain %s", assetName)
}

func extractExecutables(archiveBytes []byte, assetName string) (map[string][]byte, error) {
	files := map[string][]byte{}
	readEntry := func(name string, r io.Reader, size int64) error {
		clean := filepath.ToSlash(filepath.Clean(name))
		clean = strings.TrimPrefix(clean, "./")
		if clean != "idios" && clean != "idios.exe" && clean != "idios-tui" && clean != "idios-tui.exe" {
			return nil
		}
		base := strings.TrimSuffix(clean, ".exe")
		if size < 0 || size > 90<<20 {
			return fmt.Errorf("unexpected executable size for %s", clean)
		}
		b, err := io.ReadAll(io.LimitReader(r, 90<<20+1))
		if err != nil {
			return err
		}
		if len(b) == 0 || len(b) > 90<<20 {
			return fmt.Errorf("invalid executable payload for %s", clean)
		}
		if _, ok := files[base]; ok {
			return fmt.Errorf("duplicate %s in release archive", clean)
		}
		files[base] = b
		return nil
	}
	if strings.HasSuffix(assetName, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
		if err != nil {
			return nil, fmt.Errorf("open release ZIP: %w", err)
		}
		for _, f := range zr.File {
			if f.FileInfo().IsDir() {
				continue
			}
			name := filepath.ToSlash(f.Name)
			clean := filepath.ToSlash(filepath.Clean(name))
			if strings.Contains(clean, "../") || strings.HasPrefix(clean, "/") {
				return nil, errors.New("release ZIP contains an unsafe path")
			}
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			err = readEntry(clean, r, int64(f.UncompressedSize64))
			_ = r.Close()
			if err != nil {
				return nil, err
			}
		}
	} else if strings.HasSuffix(assetName, ".tar.gz") {
		gz, err := gzip.NewReader(bytes.NewReader(archiveBytes))
		if err != nil {
			return nil, fmt.Errorf("open release TAR.GZ: %w", err)
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("read release TAR: %w", err)
			}
			name := filepath.ToSlash(filepath.Clean(hdr.Name))
			if strings.Contains(name, "../") || strings.HasPrefix(name, "/") {
				return nil, errors.New("release TAR contains an unsafe path")
			}
			if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
				continue
			}
			if err := readEntry(name, tr, hdr.Size); err != nil {
				return nil, err
			}
		}
	} else {
		return nil, fmt.Errorf("unsupported release archive %q", assetName)
	}
	return files, nil
}

func writeStaged(dir, pattern string, contents []byte) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if _, err := f.Write(contents); err != nil {
		return "", err
	}
	mode := os.FileMode(0o755)
	if err := f.Chmod(mode); err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	ok = true
	return name, nil
}

// Schedule starts a small helper that waits for the current TUI/CLI process to exit, then
// replaces both binaries and launches the new idios command. No binary is removed before both
// replacement files have been downloaded, checksum-verified, extracted, and staged successfully.
func Schedule(prepared Prepared, parentPID int) error {
	return ScheduleWithArgs(prepared, parentPID, nil)
}

// ScheduleWithArgs stages the same verified two-binary update as Schedule, then starts the
// replacement CLI with explicitly supplied arguments after the current process exits. Args are
// quoted for the platform helper; callers must still use fixed, application-owned arguments.
func ScheduleWithArgs(prepared Prepared, parentPID int, args []string) error {
	if parentPID <= 0 {
		prepared.Cleanup()
		return errors.New("invalid parent process id")
	}
	if prepared.StagedCLI == "" || prepared.StagedTUI == "" || prepared.DestCLI == "" || prepared.DestTUI == "" {
		prepared.Cleanup()
		return errors.New("incomplete staged update")
	}
	var script string
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		script = windowsHelperWithArgs(prepared, parentPID, args)
		file, err := os.CreateTemp("", "idios-upgrade-*.ps1")
		if err != nil {
			prepared.Cleanup()
			return err
		}
		path := file.Name()
		if _, err := file.WriteString(script); err != nil {
			_ = file.Close()
			_ = os.Remove(path)
			prepared.Cleanup()
			return err
		}
		if err := file.Close(); err != nil {
			_ = os.Remove(path)
			prepared.Cleanup()
			return err
		}
		cmd = exec.Command("powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-File", path)
	} else {
		script = unixHelperWithArgs(prepared, parentPID, args)
		file, err := os.CreateTemp("", "idios-upgrade-*.sh")
		if err != nil {
			prepared.Cleanup()
			return err
		}
		path := file.Name()
		if _, err := file.WriteString(script); err != nil {
			_ = file.Close()
			_ = os.Remove(path)
			prepared.Cleanup()
			return err
		}
		if err := file.Chmod(0o700); err != nil {
			_ = file.Close()
			_ = os.Remove(path)
			prepared.Cleanup()
			return err
		}
		if err := file.Close(); err != nil {
			_ = os.Remove(path)
			prepared.Cleanup()
			return err
		}
		cmd = exec.Command("sh", path)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		prepared.Cleanup()
		return fmt.Errorf("start upgrade helper: %w", err)
	}
	return nil
}

func unixHelper(p Prepared, pid int) string {
	return unixHelperWithArgs(p, pid, nil)
}

func unixHelperWithArgs(p Prepared, pid int, args []string) string {
	launch := `"$DEST_CLI"`
	for _, arg := range args {
		launch += " " + shellQuote(arg)
	}
	return "#!/bin/sh\n" +
		"PARENT_PID=" + shellQuote(fmt.Sprint(pid)) + "\n" +
		"STAGED_CLI=" + shellQuote(p.StagedCLI) + "\n" +
		"STAGED_TUI=" + shellQuote(p.StagedTUI) + "\n" +
		"DEST_CLI=" + shellQuote(p.DestCLI) + "\n" +
		"DEST_TUI=" + shellQuote(p.DestTUI) + "\n" +
		"BACKUP_CLI=\"${DEST_CLI}.idios-backup-${PARENT_PID}\"\n" +
		"BACKUP_TUI=\"${DEST_TUI}.idios-backup-${PARENT_PID}\"\n" +
		"while kill -0 \"$PARENT_PID\" 2>/dev/null; do sleep 0.2; done\n" +
		"had_cli=0; had_tui=0; installed_cli=0; installed_tui=0\n" +
		"if [ -e \"$DEST_CLI\" ]; then mv \"$DEST_CLI\" \"$BACKUP_CLI\" || exit 1; had_cli=1; fi\n" +
		"if [ -e \"$DEST_TUI\" ]; then if ! mv \"$DEST_TUI\" \"$BACKUP_TUI\"; then [ \"$had_cli\" -eq 0 ] || mv \"$BACKUP_CLI\" \"$DEST_CLI\"; exit 1; fi; had_tui=1; fi\n" +
		"if mv \"$STAGED_CLI\" \"$DEST_CLI\"; then installed_cli=1; else [ \"$had_cli\" -eq 0 ] || mv \"$BACKUP_CLI\" \"$DEST_CLI\"; [ \"$had_tui\" -eq 0 ] || mv \"$BACKUP_TUI\" \"$DEST_TUI\"; exit 1; fi\n" +
		"if mv \"$STAGED_TUI\" \"$DEST_TUI\"; then installed_tui=1; else rm -f \"$DEST_CLI\"; [ \"$had_cli\" -eq 0 ] || mv \"$BACKUP_CLI\" \"$DEST_CLI\"; [ \"$had_tui\" -eq 0 ] || mv \"$BACKUP_TUI\" \"$DEST_TUI\"; exit 1; fi\n" +
		"chmod 755 \"$DEST_CLI\" \"$DEST_TUI\"\n" +
		launch + "\n" +
		"launch_status=$?\n" +
		"if [ \"$launch_status\" -eq 126 ] || [ \"$launch_status\" -eq 127 ]; then\n" +
		"  rm -f \"$DEST_CLI\" \"$DEST_TUI\"\n" +
		"  [ \"$had_cli\" -eq 0 ] || mv \"$BACKUP_CLI\" \"$DEST_CLI\"\n" +
		"  [ \"$had_tui\" -eq 0 ] || mv \"$BACKUP_TUI\" \"$DEST_TUI\"\n" +
		"  exit \"$launch_status\"\n" +
		"fi\n" +
		"rm -f \"$BACKUP_CLI\" \"$BACKUP_TUI\" \"$0\"\n" +
		"exit \"$launch_status\"\n"
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

func windowsHelper(p Prepared, pid int) string {
	return windowsHelperWithArgs(p, pid, nil)
}

func windowsHelperWithArgs(p Prepared, pid int, args []string) string {
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	startProcess := "  Start-Process -FilePath $destCli\n"
	if len(args) > 0 {
		quoted := make([]string, 0, len(args))
		for _, arg := range args {
			quoted = append(quoted, q(arg))
		}
		startProcess = "  $launchArgs = @(" + strings.Join(quoted, ", ") + ")\n" + "  Start-Process -FilePath $destCli -ArgumentList $launchArgs\n"
	}
	return "$ErrorActionPreference = 'Stop'\n" +
		"$parentPid = " + fmt.Sprint(pid) + "\n" +
		"$stagedCli = " + q(p.StagedCLI) + "\n" +
		"$stagedTui = " + q(p.StagedTUI) + "\n" +
		"$destCli = " + q(p.DestCLI) + "\n" +
		"$destTui = " + q(p.DestTUI) + "\n" +
		"$backupCli = \"$destCli.idios-backup-$parentPid\"\n" +
		"$backupTui = \"$destTui.idios-backup-$parentPid\"\n" +
		"while (Get-Process -Id $parentPid -ErrorAction SilentlyContinue) { Start-Sleep -Milliseconds 200 }\n" +
		"$hadCli = Test-Path -LiteralPath $destCli; $hadTui = Test-Path -LiteralPath $destTui\n" +
		"$installedCli = $false; $installedTui = $false\n" +
		"try {\n" +
		"  if ($hadCli) { Move-Item -LiteralPath $destCli -Destination $backupCli -Force }\n" +
		"  if ($hadTui) { Move-Item -LiteralPath $destTui -Destination $backupTui -Force }\n" +
		"  Move-Item -LiteralPath $stagedCli -Destination $destCli -Force\n" +
		"  $installedCli = $true\n" +
		"  Move-Item -LiteralPath $stagedTui -Destination $destTui -Force\n" +
		"  $installedTui = $true\n" +
		startProcess +
		"  if (Test-Path -LiteralPath $backupCli) { Remove-Item -LiteralPath $backupCli -Force -ErrorAction SilentlyContinue }\n" +
		"  if (Test-Path -LiteralPath $backupTui) { Remove-Item -LiteralPath $backupTui -Force -ErrorAction SilentlyContinue }\n" +
		"} catch {\n" +
		"  if ($installedCli -and (Test-Path -LiteralPath $destCli)) { Remove-Item -LiteralPath $destCli -Force -ErrorAction SilentlyContinue }\n" +
		"  if ($installedTui -and (Test-Path -LiteralPath $destTui)) { Remove-Item -LiteralPath $destTui -Force -ErrorAction SilentlyContinue }\n" +
		"  if ($hadCli -and (Test-Path -LiteralPath $backupCli)) { Move-Item -LiteralPath $backupCli -Destination $destCli -Force -ErrorAction SilentlyContinue }\n" +
		"  if ($hadTui -and (Test-Path -LiteralPath $backupTui)) { Move-Item -LiteralPath $backupTui -Destination $destTui -Force -ErrorAction SilentlyContinue }\n" +
		"  Write-Error $_; exit 1\n" +
		"}\n"
}
