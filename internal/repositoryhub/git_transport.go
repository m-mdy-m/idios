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

func sourceForgeTreePage(ctx context.Context, token string, repo RemoteRepository, ref, path string, page int) (TreePage, error) {
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		return TreePage{}, fmt.Errorf("%w: tree page is out of range", domain.ErrInvalid)
	}
	var result TreePage
	err := withSourceForgeClone(ctx, token, repo, ref, func(dir string) error {
		treeRef := "HEAD"
		if path != "" {
			if !safeGitPath(path) {
				return fmt.Errorf("%w: safe repository directory path is required", domain.ErrInvalid)
			}
			resolved, err := runGit(ctx, token, dir, "rev-parse", "--verify", "HEAD:"+path)
			if err != nil {
				return fmt.Errorf("%w: SourceForge directory %q was not found", domain.ErrNotFound, path)
			}
			treeRef = strings.TrimSpace(string(resolved))
		}
		raw, err := runGit(ctx, token, dir, "ls-tree", "--long", "-z", treeRef)
		if err != nil {
			return err
		}
		entries := parseSourceForgeTreeOutput(raw, path)
		const pageSize = 100
		start := (page - 1) * pageSize
		if start > len(entries) {
			start = len(entries)
		}
		end := start + pageSize
		if end > len(entries) {
			end = len(entries)
		}
		result = TreePage{
			Items: append([]TreeEntry(nil), entries[start:end]...),
			Page:  page, HasMore: end < len(entries),
		}
		return nil
	})
	return result, err
}

func parseSourceForgeTreeOutput(raw []byte, parent string) []TreeEntry {
	out := make([]TreeEntry, 0)
	for _, record := range strings.Split(string(raw), "\x00") {
		if record == "" {
			continue
		}
		parts := strings.SplitN(record, "\t", 2)
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
		typ := meta[1]
		if typ != "tree" {
			typ = "blob"
		}
		name := parts[1]
		out = append(out, TreeEntry{
			Path: joinTreePath(parent, name), Name: name,
			Type: typ, SHA: meta[2], Size: size,
		})
	}
	return out
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

func sourceForgeTags(ctx context.Context, token string, repo RemoteRepository, page, pageSize int) (TagPage, error) {
	if page < 1 {
		page = 1
	}
	if page > 200 {
		return TagPage{}, fmt.Errorf("%w: SourceForge tag page is out of range", domain.ErrInvalid)
	}
	result := TagPage{
		Items: make([]Tag, 0),
		Page:  page,
		Availability: FeatureAvailability{
			Available: true,
			State:     "available",
		},
	}
	err := withSourceForgeClone(ctx, token, repo, repo.DefaultBranch, func(dir string) error {
		if _, err := runGit(ctx, token, dir, "fetch", "--quiet", "--filter=blob:none", "--tags", "origin"); err != nil {
			return err
		}
		limit := page*pageSize + 1
		raw, err := runGit(
			ctx, token, dir,
			"for-each-ref", "--sort=-creatordate", "--count="+strconv.Itoa(limit),
			"--format=%(refname:strip=2)%09%(objectname)%09%(objecttype)%09%(*objectname)%09%(creatordate:iso-strict)",
			"refs/tags",
		)
		if err != nil {
			return err
		}
		records := parseSourceForgeTagRefs(raw)
		start := (page - 1) * pageSize
		end := start + pageSize
		result.HasMore = len(records) > end
		if start >= len(records) {
			return nil
		}
		if end > len(records) {
			end = len(records)
		}
		for _, record := range records[start:end] {
			if !validRemoteTagName(record.Name) {
				continue
			}
			createdAt := parseSourceForgeTime(record.CreatedAt)
			item := Tag{
				Name:       record.Name,
				ObjectSHA:  record.ObjectSHA,
				ObjectType: record.ObjectType,
				TargetSHA:  record.PeeledSHA,
				URL:        providerDownloadURL(strings.TrimRight(repo.WebURL, "/") + "/tree/" + url.PathEscape(record.Name)),
			}
			if record.ObjectType == "tag" {
				item.Kind = "annotated"
				item.CreatedAt = createdAt
				annotation, targetSHA, targetType, err := sourceForgeAnnotatedTag(ctx, token, dir, record.ObjectSHA)
				if err != nil {
					return err
				}
				item.Annotation = annotation
				if targetSHA != "" {
					item.TargetSHA = targetSHA
				}
				if targetType != "" {
					item.ObjectType = targetType
				}
			} else {
				item.Kind = "lightweight"
				if item.TargetSHA == "" {
					item.TargetSHA = record.ObjectSHA
				}
				item.CommitCreatedAt = createdAt
			}
			if item.TargetSHA != "" {
				committed, err := runGit(ctx, token, dir, "show", "-s", "--format=%cI", item.TargetSHA)
				if err == nil {
					item.CommitCreatedAt = parseSourceForgeTime(strings.TrimSpace(string(committed)))
				}
			}
			result.Items = append(result.Items, item)
		}
		return nil
	})
	return result, err
}

type sourceForgeTagRef struct {
	Name       string
	ObjectSHA  string
	ObjectType string
	PeeledSHA  string
	CreatedAt  string
}

func parseSourceForgeTagRefs(raw []byte) []sourceForgeTagRef {
	records := make([]sourceForgeTagRef, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 5)
		if len(fields) < 4 || fields[0] == "" || fields[1] == "" {
			continue
		}
		record := sourceForgeTagRef{
			Name: fields[0], ObjectSHA: fields[1], ObjectType: fields[2], PeeledSHA: fields[3],
		}
		if len(fields) == 5 {
			record.CreatedAt = fields[4]
		}
		records = append(records, record)
	}
	return records
}

func parseSourceForgeTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	return &parsed
}

func sourceForgeAnnotatedTag(ctx context.Context, token, dir, objectSHA string) (string, string, string, error) {
	current := objectSHA
	annotation := ""
	targetSHA := ""
	targetType := ""
	for depth := 0; depth < 5; depth++ {
		raw, err := runGit(ctx, token, dir, "cat-file", "-p", current)
		if err != nil {
			return "", "", "", err
		}
		header, body, found := strings.Cut(string(raw), "\n\n")
		if !found {
			return "", "", "", fmt.Errorf("%w: SourceForge returned an invalid annotated tag object", domain.ErrUnavailable)
		}
		var object, objectType string
		for _, line := range strings.Split(header, "\n") {
			if strings.HasPrefix(line, "object ") {
				object = strings.TrimSpace(strings.TrimPrefix(line, "object "))
			}
			if strings.HasPrefix(line, "type ") {
				objectType = strings.TrimSpace(strings.TrimPrefix(line, "type "))
			}
		}
		if object == "" || objectType == "" {
			return "", "", "", fmt.Errorf("%w: SourceForge annotated tag has incomplete target metadata", domain.ErrUnavailable)
		}
		if depth == 0 {
			annotation = strings.TrimSpace(body)
		}
		if objectType != "tag" {
			targetSHA, targetType = object, objectType
			return annotation, targetSHA, targetType, nil
		}
		current = object
	}
	return "", "", "", fmt.Errorf("%w: SourceForge tag object nesting exceeds the supported limit", domain.ErrUnavailable)
}

func sourceForgeCompareTags(ctx context.Context, token string, repo RemoteRepository, from, to string) (TagComparison, error) {
	var result TagComparison
	err := withSourceForgeClone(ctx, token, repo, repo.DefaultBranch, func(dir string) error {
		fromRef := "refs/tags/" + from
		toRef := "refs/tags/" + to
		for _, ref := range []string{fromRef, toRef} {
			if _, err := runGit(ctx, token, dir, "check-ref-format", ref); err != nil {
				return fmt.Errorf("%w: invalid SourceForge tag reference", domain.ErrInvalid)
			}
		}
		if _, err := runGit(ctx, token, dir, "fetch", "--quiet", "--filter=blob:none", "--deepen=10000", "origin",
			fromRef+":"+fromRef, toRef+":"+toRef); err != nil {
			return err
		}
		fromCommit, err := runGit(ctx, token, dir, "rev-parse", "--verify", fromRef+"^{commit}")
		if err != nil {
			return fmt.Errorf("%w: SourceForge tag %q does not resolve to a commit", domain.ErrNotFound, from)
		}
		toCommit, err := runGit(ctx, token, dir, "rev-parse", "--verify", toRef+"^{commit}")
		if err != nil {
			return fmt.Errorf("%w: SourceForge tag %q does not resolve to a commit", domain.ErrNotFound, to)
		}
		baseSHA, err := runGit(ctx, token, dir, "merge-base", fromRef, toRef)
		if err != nil {
			return fmt.Errorf("%w: SourceForge tag histories do not have a known common ancestor; deepen the clone and retry", domain.ErrUnavailable)
		}
		aheadRaw, err := runGit(ctx, token, dir, "rev-list", "--count", fromRef+".."+toRef)
		if err != nil {
			return err
		}
		behindRaw, err := runGit(ctx, token, dir, "rev-list", "--count", toRef+".."+fromRef)
		if err != nil {
			return err
		}
		ahead, _ := strconv.Atoi(strings.TrimSpace(string(aheadRaw)))
		behind, _ := strconv.Atoi(strings.TrimSpace(string(behindRaw)))
		state := "diverged"
		switch {
		case ahead == 0 && behind == 0:
			state = "identical"
		case ahead > 0 && behind == 0:
			state = "ahead"
		case ahead == 0 && behind > 0:
			state = "behind"
		}
		result = TagComparison{
			From: from, To: to, State: state,
			CountsAvailable: true,
			MergeBaseSHA:    strings.TrimSpace(string(baseSHA)),
			AheadBy:         ahead, BehindBy: behind, TotalCommits: ahead,
			Commits:      make([]TagCompareCommit, 0),
			Files:        make([]TagCompareFile, 0),
			Availability: FeatureAvailability{Available: true, State: "available"},
		}
		log, err := runGit(ctx, token, dir, "log", "--max-count=100",
			"--format=%H%x1f%s%x1f%an%x1f%cn%x1f%cI", fromRef+".."+toRef)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(log)), "\n") {
			if line == "" {
				continue
			}
			fields := strings.SplitN(line, "\x1f", 5)
			if len(fields) != 5 {
				continue
			}
			result.Commits = append(result.Commits, TagCompareCommit{
				SHA: fields[0], Title: fields[1], Author: fields[2], Committer: fields[3],
				CommittedAt: parseSourceForgeTime(fields[4]),
			})
		}
		result.CommitsTruncated = ahead > len(result.Commits)

		names, err := runGit(ctx, token, dir, "diff", "--name-status", "-M", fromRef+"..."+toRef)
		if err != nil {
			return err
		}
		numstats, err := runGit(ctx, token, dir, "diff", "--numstat", "-M", fromRef+"..."+toRef)
		if err != nil {
			return err
		}
		statsByPath := make(map[string][2]int)
		for _, line := range strings.Split(strings.TrimSpace(string(numstats)), "\n") {
			if line == "" {
				continue
			}
			fields := strings.SplitN(line, "\t", 3)
			if len(fields) != 3 {
				continue
			}
			additions, _ := strconv.Atoi(fields[0])
			deletions, _ := strconv.Atoi(fields[1])
			statsByPath[fields[2]] = [2]int{additions, deletions}
		}
		for _, line := range strings.Split(strings.TrimSpace(string(names)), "\n") {
			if line == "" {
				continue
			}
			fields := strings.SplitN(line, "\t", 3)
			if len(fields) < 2 {
				continue
			}
			status := "modified"
			path := fields[len(fields)-1]
			previous := ""
			switch {
			case strings.HasPrefix(fields[0], "A"):
				status = "added"
			case strings.HasPrefix(fields[0], "D"):
				status = "deleted"
			case strings.HasPrefix(fields[0], "R"):
				status = "renamed"
				if len(fields) == 3 {
					previous = fields[1]
				}
			}
			counts := statsByPath[path]
			result.Files = append(result.Files, TagCompareFile{
				Path: path, PreviousPath: previous, Status: status,
				Additions: counts[0], Deletions: counts[1],
			})
		}
		if len(result.Files) > 300 {
			result.Files = result.Files[:300]
			result.FilesTruncated = true
		}
		_ = fromCommit
		_ = toCommit
		return nil
	})
	return result, err
}
