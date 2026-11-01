package repositoryhub

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"idios/internal/domain"
)

const maxGitOutputBytes = 16 << 20

func SourceForgeSSHCloneURL(accountName, fullPath string) (string, error) {
	if accountName == "" || strings.TrimSpace(accountName) != accountName {
		return "", fmt.Errorf("%w: SourceForge account name must be non-empty and contain no surrounding whitespace", domain.ErrInvalid)
	}
	project, repository, ok := strings.Cut(strings.Trim(fullPath, "/"), "/")
	if accountName == "" || !ok || project == "" || repository == "" ||
		strings.ContainsAny(accountName+project+repository, "/\\\\") ||
		project == "." || project == ".." || repository == "." || repository == ".." {
		return "", fmt.Errorf("%w: SourceForge account name and project/repository path are required", domain.ErrInvalid)
	}
	for _, part := range []string{accountName, project, repository} {
		if strings.TrimSpace(part) != part || strings.ContainsAny(part, " \t\r\n") {
			return "", fmt.Errorf("%w: SourceForge SSH identity contains invalid whitespace", domain.ErrInvalid)
		}
	}
	target := &url.URL{
		Scheme: "ssh",
		User:   url.User(accountName),
		Host:   "git.code.sf.net",
		Path:   "/p/" + project + "/" + repository,
	}
	return target.String(), nil
}

func sourceForgeCloneURL(repo RemoteRepository) (string, error) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("IDIOS_SOURCEFORGE_GIT_TRANSPORT")), "ssh") {
		raw := strings.TrimSpace(repo.CloneSSHURL)
		if raw == "" {
			return "", fmt.Errorf("%w: SSH transport was selected but no SourceForge SSH clone URL is configured", domain.ErrUnavailable)
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed == nil || parsed.User == nil {
			return "", fmt.Errorf("%w: SourceForge SSH clone URL is invalid", domain.ErrInvalid)
		}
		_, hasPassword := parsed.User.Password()
		if parsed.Scheme != "ssh" || !strings.EqualFold(parsed.Hostname(), "git.code.sf.net") ||
			parsed.User.Username() == "" || hasPassword ||
			(repo.SSHAccountName == "" || parsed.User.Username() != repo.SSHAccountName) ||
			(parsed.Port() != "" && parsed.Port() != "22") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", fmt.Errorf("%w: SourceForge SSH clone URL is invalid", domain.ErrInvalid)
		}
		if !safeGitPath(strings.TrimPrefix(parsed.Path, "/")) {
			return "", fmt.Errorf("%w: SourceForge SSH clone path is invalid", domain.ErrInvalid)
		}
		return parsed.String(), nil
	}
	raw := strings.TrimSpace(repo.CloneURL)
	if raw == "" {
		return "", fmt.Errorf("%w: SourceForge did not return an anonymous HTTPS Git clone URL", domain.ErrUnsupported)
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%w: SourceForge Git transport requires a clean HTTPS clone URL", domain.ErrInvalid)
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "sourceforge.net" && host != "git.code.sf.net" {
		return "", fmt.Errorf("%w: SourceForge clone host %q is not trusted", domain.ErrInvalid, host)
	}
	if parsed.Port() != "" && parsed.Port() != "443" {
		return "", fmt.Errorf("%w: non-standard Git clone ports are not allowed", domain.ErrInvalid)
	}
	return parsed.String(), nil
}

func runGit(ctx context.Context, _ string, dir string, args ...string) ([]byte, error) {
	binary, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("%w: SourceForge code browsing requires the Git executable on the IDIOS host", domain.ErrUnavailable)
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	env := []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("IDIOS_SOURCEFORGE_GIT_TRANSPORT")), "ssh") {
		// Use the user's existing OpenSSH agent/configuration. Do not prompt or trust a
		// previously unknown host key automatically; known_hosts must be configured first.
		env = append(env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes -o StrictHostKeyChecking=yes")
	}
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%w: Git transport timed out or was cancelled", domain.ErrUnavailable)
	}
	if len(out) > maxGitOutputBytes {
		return nil, fmt.Errorf("%w: Git output exceeded 16 MiB", domain.ErrUnavailable)
	}
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			message = err.Error()
		}
		lower := strings.ToLower(message)
		if strings.Contains(lower, "authentication failed") || strings.Contains(lower, "401") || strings.Contains(lower, "403") {
			return nil, fmt.Errorf("%w: SourceForge Git transport denied access; the API bearer token may not grant Git push/clone rights", domain.ErrPermission)
		}
		if strings.Contains(lower, "not found") || strings.Contains(lower, "repository does not exist") {
			return nil, fmt.Errorf("%w: SourceForge Git repository could not be found", domain.ErrNotFound)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("%w: Git transport failed: %s", domain.ErrUnavailable, boundedText(message, 1200))
		}
		return nil, fmt.Errorf("%w: Git transport failed: %v", domain.ErrUnavailable, err)
	}
	return out, nil
}

func boundedText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}

func withSourceForgeClone(ctx context.Context, token string, repo RemoteRepository, ref string, fn func(string) error) error {
	cloneURL, err := sourceForgeCloneURL(repo)
	if err != nil {
		return err
	}
	if ref == "" {
		ref = repo.DefaultBranch
	}
	if ref == "" {
		return fmt.Errorf("%w: choose a SourceForge Git branch first", domain.ErrInvalid)
	}
	temp, err := os.MkdirTemp("", "idios-hub-git-*")
	if err != nil {
		return fmt.Errorf("%w: could not create temporary Git workspace", domain.ErrUnavailable)
	}
	defer os.RemoveAll(temp)
	repoDir := filepath.Join(temp, "repo")
	_, err = runGit(ctx, token, "", "clone", "--quiet", "--filter=blob:none", "--depth=1", "--single-branch", "--branch", ref, cloneURL, repoDir)
	if err != nil {
		return err
	}
	return fn(repoDir)
}

func sourceForgeBranches(ctx context.Context, token string, repo RemoteRepository) ([]Branch, error) {
	cloneURL, err := sourceForgeCloneURL(repo)
	if err != nil {
		return nil, err
	}
	head, err := runGit(ctx, token, "", "ls-remote", "--symref", cloneURL, "HEAD")
	if err != nil {
		return nil, err
	}
	defaultBranch := repo.DefaultBranch
	for _, line := range strings.Split(string(head), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "ref:" && fields[2] == "HEAD" {
			defaultBranch = strings.TrimPrefix(fields[1], "refs/heads/")
		}
	}
	raw, err := runGit(ctx, token, "", "ls-remote", "--heads", cloneURL)
	if err != nil {
		return nil, err
	}
	out := make([]Branch, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !strings.HasPrefix(fields[1], "refs/heads/") {
			continue
		}
		name := strings.TrimPrefix(fields[1], "refs/heads/")
		out = append(out, Branch{Name: name, CommitSHA: fields[0], Default: name == defaultBranch})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no Git branches were returned by SourceForge", domain.ErrNotFound)
	}
	if defaultBranch == "" {
		out[0].Default = true
	}
	return out, nil
}

func sourceForgeTree(ctx context.Context, token string, repo RemoteRepository, ref string) ([]TreeEntry, error) {
	var out []TreeEntry
	err := withSourceForgeClone(ctx, token, repo, ref, func(dir string) error {
		raw, err := runGit(ctx, token, dir, "ls-tree", "-r", "-t", "--long", "--full-tree", "HEAD")
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) != 2 {
				continue
			}
			meta := strings.Fields(parts[0])
			if len(meta) < 3 {
				continue
			}
			size := int64(0)
			if len(meta) >= 4 && meta[3] != "-" {
				size, _ = strconv.ParseInt(meta[3], 10, 64)
			}
			out = append(out, TreeEntry{Path: parts[1], Name: lastPath(parts[1]), Type: meta[1], SHA: meta[2], Size: size})
		}
		return nil
	})
	return out, err
}

func sourceForgeFile(ctx context.Context, token string, repo RemoteRepository, path, ref string) (File, error) {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if !safeGitPath(path) {
		return File{}, fmt.Errorf("%w: safe repository file path is required", domain.ErrInvalid)
	}
	var result File
	err := withSourceForgeClone(ctx, token, repo, ref, func(dir string) error {
		revision := "HEAD:" + path
		sha, err := runGit(ctx, token, dir, "rev-parse", revision)
		if err != nil {
			return fmt.Errorf("%w: SourceForge file %q", domain.ErrNotFound, path)
		}
		content, err := runGit(ctx, token, dir, "show", revision)
		if err != nil {
			return err
		}
		if len(content) > maxResponseBytes {
			return fmt.Errorf("%w: SourceForge file exceeds 8 MiB", domain.ErrUnavailable)
		}
		result = File{Path: path, Content: string(content), SHA: strings.TrimSpace(string(sha)), Size: int64(len(content)), Encoding: "utf-8"}
		return nil
	})
	return result, err
}

func sourceForgeCommits(ctx context.Context, token string, repo RemoteRepository, ref string, page int) ([]Commit, error) {
	if page < 1 {
		page = 1
	}
	if page > 200 {
		return nil, fmt.Errorf("%w: commit page is out of range", domain.ErrInvalid)
	}
	if ref == "" {
		ref = repo.DefaultBranch
	}
	var out []Commit
	err := withSourceForgeClone(ctx, token, repo, ref, func(dir string) error {
		count := page * 50
		if count > 1 {
			if _, err := runGit(ctx, token, dir, "fetch", "--quiet", "--deepen="+strconv.Itoa(count-1), "origin", ref); err != nil {
				return err
			}
		}
		raw, err := runGit(ctx, token, dir, "log", "-n", strconv.Itoa(count), "--format=%H%x1f%an%x1f%ae%x1f%aI%x1f%cn%x1f%ce%x1f%cI%x1f%s", "HEAD")
		if err != nil {
			return err
		}
		lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
		start := (page - 1) * 50
		for i, line := range lines {
			if i < start {
				continue
			}
			if len(out) >= 50 {
				break
			}
			fields := strings.SplitN(line, "\x1f", 8)
			if len(fields) != 8 {
				continue
			}
			authored, _ := time.Parse(time.RFC3339, fields[3])
			committed, _ := time.Parse(time.RFC3339, fields[6])
			out = append(out, Commit{
				SHA: fields[0], Author: fields[1], Email: fields[2], AuthoredAt: optionalTime(authored),
				Committer: fields[4], CommitterEmail: fields[5], Committed: committed, Message: fields[7],
			})
		}
		return nil
	})
	return out, err
}

func sourceForgeCommit(ctx context.Context, token string, repo RemoteRepository, ref, sha string) (Commit, error) {
	if !validCommitSHA(sha) {
		return Commit{}, fmt.Errorf("%w: a full commit SHA is required", domain.ErrInvalid)
	}
	if ref == "" {
		ref = repo.DefaultBranch
	}
	var result Commit
	err := withSourceForgeClone(ctx, token, repo, ref, func(dir string) error {
		if _, err := runGit(ctx, token, dir, "fetch", "--quiet", "--deepen=10000", "origin", ref); err != nil {
			return err
		}
		raw, err := runGit(ctx, token, dir, "show", "-s", "--format=%H%x1f%an%x1f%ae%x1f%aI%x1f%cn%x1f%ce%x1f%cI%x1f%B", sha)
		if err != nil {
			return fmt.Errorf("%w: SourceForge commit %q was not found in the selected branch history", domain.ErrNotFound, sha)
		}
		fields := strings.SplitN(strings.TrimSpace(string(raw)), "\x1f", 8)
		if len(fields) != 8 {
			return fmt.Errorf("%w: SourceForge returned incomplete commit metadata", domain.ErrUnavailable)
		}
		authored, _ := time.Parse(time.RFC3339, fields[3])
		committed, _ := time.Parse(time.RFC3339, fields[6])
		result = Commit{
			SHA: fields[0], Message: strings.TrimSpace(fields[7]),
			Author: fields[1], Email: fields[2], AuthoredAt: optionalTime(authored),
			Committer: fields[4], CommitterEmail: fields[5], Committed: committed,
			SignatureSource: "local", ChecksAvailable: false, ChecksState: "unavailable",
		}

		object, err := runGit(ctx, token, dir, "cat-file", "-p", sha)
		if err != nil {
			return err
		}
		signature := gitCommitSignature(object)
		if signature == "" {
			result.SignatureStatus = "unsigned"
			result.SignatureReason = "No cryptographic signature is stored in this Git commit."
		} else {
			result.SignatureType = detectCommitSignatureType(signature)
			verifyArgs := []string{"verify-commit", "--raw", sha}
			if result.SignatureType == "SSH" {
				if allowedSigners := strings.TrimSpace(os.Getenv("IDIOS_SOURCEFORGE_SSH_ALLOWED_SIGNERS_FILE")); allowedSigners != "" {
					verifyArgs = append([]string{"-c", "gpg.ssh.allowedSignersFile=" + allowedSigners}, verifyArgs...)
				}
			}
			if _, verifyErr := runGit(ctx, token, dir, verifyArgs...); verifyErr == nil {
				result.SignatureStatus = "verified"
				result.SignatureReason = "Local Git signature verification succeeded."
			} else {
				result.SignatureStatus = "unverified"
				result.SignatureReason = "Local Git could not verify the commit signature with the configured keyring or SSH allowed-signers file."
			}
		}

		numstat, err := runGit(ctx, token, dir, "show", "--numstat", "--format=", "--find-renames", sha)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(numstat)), "\n") {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) != 3 {
				continue
			}
			additions, _ := strconv.Atoi(parts[0])
			deletions, _ := strconv.Atoi(parts[1])
			result.Files = append(result.Files, CommitFile{
				Path: parts[2], Status: "modified", Additions: additions, Deletions: deletions,
			})
			result.Stats.Additions += additions
			result.Stats.Deletions += deletions
		}
		result.Stats.Total = result.Stats.Additions + result.Stats.Deletions

		nameStatus, err := runGit(ctx, token, dir, "diff-tree", "--root", "--no-commit-id", "--name-status", "-r", "-M", sha)
		if err == nil {
			statuses := strings.Split(strings.TrimSpace(string(nameStatus)), "\n")
			for index, line := range statuses {
				fields := strings.SplitN(line, "\t", 3)
				if len(fields) < 2 || index >= len(result.Files) {
					continue
				}
				file := &result.Files[index]
				switch {
				case strings.HasPrefix(fields[0], "A"):
					file.Status = "added"
				case strings.HasPrefix(fields[0], "D"):
					file.Status = "deleted"
				case strings.HasPrefix(fields[0], "R") && len(fields) == 3:
					file.Status = "renamed"
					file.PreviousPath = fields[1]
					file.Path = fields[2]
				default:
					file.Path = fields[len(fields)-1]
				}
			}
		}

		patch, patchErr := runGit(ctx, token, dir, "show", "--format=", "--find-renames", "--patch", sha)
		if patchErr != nil {
			result.DiffUnavailableReason = patchErr.Error()
		} else if len(patch) > maxResponseBytes {
			result.DiffUnavailableReason = "Commit diff exceeds the 8 MiB preview limit."
		} else {
			result.Diff = string(patch)
			result.DiffAvailable = true
		}
		return nil
	})
	return result, err
}

func gitCommitSignature(object []byte) string {
	var signature strings.Builder
	lines := strings.Split(string(object), "\n")
	inHeaders := true
	inSignature := false
	for _, line := range lines {
		if inHeaders && line == "" {
			break
		}
		if strings.HasPrefix(line, "gpgsig ") || strings.HasPrefix(line, "gpgsig-sha256 ") {
			inSignature = true
			signature.WriteString(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "gpgsig "), "gpgsig-sha256 ")))
			signature.WriteByte('\n')
			continue
		}
		if inSignature && strings.HasPrefix(line, " ") {
			signature.WriteString(strings.TrimSpace(line))
			signature.WriteByte('\n')
			continue
		}
		inSignature = false
	}
	return signature.String()
}

func safeGitPath(path string) bool {
	if path == "" || strings.ContainsRune(path, 0) {
		return false
	}
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return !filepath.IsAbs(path) && !strings.HasPrefix(path, "/")
}
