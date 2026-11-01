// Package localgit provides provider-independent operations on real local Git working copies.
package localgit

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	maxCommandOutput = 8 << 20
	maxDiffOutput    = 2 << 20
	commandTimeout   = 2 * time.Minute
)

type Availability struct {
	Available  bool   `json:"available"`
	Backend    string `json:"backend"`
	Executable string `json:"executable,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type Status struct {
	Available   bool   `json:"available"`
	Path        string `json:"path,omitempty"`
	Initialized bool   `json:"initialized"`
	Backend     string `json:"backend"`
	Reason      string `json:"reason,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Clean       bool   `json:"clean"`
	Staged      int    `json:"staged"`
	Unstaged    int    `json:"unstaged"`
	Untracked   int    `json:"untracked"`
}

type Branch struct {
	Name      string `json:"name"`
	CommitSHA string `json:"commit_sha"`
	Current   bool   `json:"current"`
}

type Tag struct {
	Name      string `json:"name"`
	TargetSHA string `json:"target_sha"`
	Annotated bool   `json:"annotated"`
	Message   string `json:"message,omitempty"`
}

type Commit struct {
	SHA         string `json:"sha"`
	Author      string `json:"author"`
	AuthorEmail string `json:"author_email"`
	AuthorDate  string `json:"author_date"`
	Committer   string `json:"committer"`
	CommitEmail string `json:"committer_email"`
	CommitDate  string `json:"commit_date"`
	Message     string `json:"message"`
}

// Engine is deliberately independent of remote provider adapters. A platform can
// implement this interface without changing the Application or HTTP contracts.
type Engine interface {
	Availability() Availability
	Init(context.Context, string, string) error
	Clone(context.Context, string, string, string) error
	Status(context.Context, string) (Status, error)
	Branches(context.Context, string) ([]Branch, error)
	CreateBranch(context.Context, string, string, string) error
	SwitchBranch(context.Context, string, string) error
	Stage(context.Context, string, []string) error
	Commit(context.Context, string, string, string, string) (Commit, error)
	Log(context.Context, string, int) ([]Commit, error)
	Tags(context.Context, string) ([]Tag, error)
	CreateTag(context.Context, string, string, string) error
	Diff(context.Context, string, bool) (string, error)
}

type CLIEngine struct{}

func NewCLIEngine() *CLIEngine { return &CLIEngine{} }

func (e *CLIEngine) Availability() Availability {
	binary, err := exec.LookPath("git")
	if err != nil {
		return Availability{Available: false, Backend: "git-cli", Reason: "Git executable was not found on this device."}
	}
	return Availability{Available: true, Backend: "git-cli", Executable: binary}
}

func (e *CLIEngine) run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	availability := e.Availability()
	if !availability.Available {
		return nil, errors.New(availability.Reason)
	}
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, availability.Executable, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	// Keep the user's Git identity and credential helpers, but never allow an
	// inherited repository/index override to redirect a local operation elsewhere.
	blocked := map[string]bool{
		"GIT_DIR":                          true,
		"GIT_WORK_TREE":                    true,
		"GIT_INDEX_FILE":                   true,
		"GIT_OBJECT_DIRECTORY":             true,
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
		"GIT_COMMON_DIR":                   true,
	}
	identityOverrides := map[string]bool{
		"GIT_AUTHOR_NAME": true, "GIT_AUTHOR_EMAIL": true,
		"GIT_COMMITTER_NAME": true, "GIT_COMMITTER_EMAIL": true,
		"GIT_AUTHOR_DATE": true, "GIT_COMMITTER_DATE": true, "EMAIL": true,
	}
	env := make([]string, 0, len(os.Environ())+2)
	for _, item := range os.Environ() {
		key, value, _ := strings.Cut(item, "=")
		if blocked[key] || key == "GIT_TERMINAL_PROMPT" {
			continue
		}
		// Empty identity overrides take precedence over git -c and user config,
		// producing an invalid blank author/committer. Preserve valid user values.
		if identityOverrides[strings.ToUpper(key)] && strings.TrimSpace(value) == "" {
			continue
		}
		env = append(env, item)
	}
	cmd.Env = append(env, "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("Git command timed out or was cancelled")
	}
	if len(output) > maxCommandOutput {
		return nil, fmt.Errorf("Git command output exceeded the 8 MiB safety limit")
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		if len(message) > 1200 {
			message = message[:1200] + "…"
		}
		return nil, fmt.Errorf("git %s: %s", args[0], message)
	}
	return output, nil
}

func absoluteDirectory(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("a managed working-copy directory is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve working-copy directory: %w", err)
	}
	return filepath.Clean(absolute), nil
}

func validateBranchName(ctx context.Context, e *CLIEngine, name string) error {
	raw := name
	name = strings.TrimSpace(name)
	if name == "" || raw != name || len(name) > 240 {
		return errors.New("a valid branch name is required")
	}
	if _, err := e.run(ctx, "", "check-ref-format", "--branch", name); err != nil {
		return fmt.Errorf("invalid branch name: %w", err)
	}
	return nil
}

func (e *CLIEngine) Init(ctx context.Context, directory, branch string) error {
	path, err := absoluteDirectory(directory)
	if err != nil {
		return err
	}
	if branch == "" {
		branch = "main"
	}
	if err := validateBranchName(ctx, e, branch); err != nil {
		return err
	}
	if info, statErr := os.Lstat(path); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("working-copy path must not be a symbolic link")
		}
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return fmt.Errorf("inspect working-copy directory: %w", readErr)
		}
		if len(entries) != 0 {
			return errors.New("working-copy directory is not empty")
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("inspect working-copy directory: %w", statErr)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create working-copy directory: %w", err)
	}
	if _, err := e.run(ctx, "", "init", "--quiet", path); err != nil {
		return err
	}
	_, err = e.run(ctx, path, "symbolic-ref", "HEAD", "refs/heads/"+branch)
	return err
}

var scpRemotePattern = regexp.MustCompile(`^[A-Za-z0-9_.+-]+@[A-Za-z0-9.-]+:[^[:space:]]+$`)

func validateRemoteURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n") {
		return errors.New("a remote HTTPS or SSH clone URL is required")
	}
	if scpRemotePattern.MatchString(raw) {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return errors.New("clone URL is invalid")
	}
	if (parsed.Scheme != "https" && parsed.Scheme != "ssh") || parsed.Host == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("clone URL must use HTTPS or SSH and must not embed credentials, query strings or fragments")
	}
	return nil
}

func (e *CLIEngine) Clone(ctx context.Context, remoteURL, directory, branch string) error {
	if err := validateRemoteURL(remoteURL); err != nil {
		return err
	}
	path, err := absoluteDirectory(directory)
	if err != nil {
		return err
	}
	if branch != "" {
		if err := validateBranchName(ctx, e, branch); err != nil {
			return err
		}
	}
	if info, statErr := os.Lstat(path); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("working-copy path must not be a symbolic link")
		}
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return fmt.Errorf("inspect working-copy directory: %w", readErr)
		}
		if len(entries) != 0 {
			return errors.New("working-copy directory already exists and is not empty")
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("prepare working-copy destination: %w", err)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("inspect working-copy directory: %w", statErr)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create working-copy parent directory: %w", err)
	}
	args := []string{"clone", "--quiet"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, "--", strings.TrimSpace(remoteURL), path)
	if _, err := e.run(ctx, "", args...); err != nil {
		_ = os.RemoveAll(path)
		return err
	}
	return nil
}

func (e *CLIEngine) isWorktree(ctx context.Context, directory string) bool {
	path, err := absoluteDirectory(directory)
	if err != nil {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false
	}
	output, err := e.run(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	topLevel, err := filepath.Abs(strings.TrimSpace(string(output)))
	if err != nil {
		return false
	}
	topInfo, topErr := os.Stat(topLevel)
	pathInfo, pathErr := os.Stat(path)
	if topErr != nil || pathErr != nil {
		return false
	}
	// A path nested inside another repository is not an independent working copy.
	// Refuse it so Stage/Commit never accidentally operate on the parent repository.
	return os.SameFile(topInfo, pathInfo)
}

func (e *CLIEngine) Status(ctx context.Context, directory string) (Status, error) {
	availability := e.Availability()
	status := Status{Available: availability.Available, Backend: availability.Backend, Reason: availability.Reason}
	if !availability.Available {
		return status, nil
	}
	if !e.isWorktree(ctx, directory) {
		return status, nil
	}
	status.Initialized = true
	output, err := e.run(ctx, directory, "status", "--porcelain=v1", "--branch", "--untracked-files=all")
	if err != nil {
		return Status{}, err
	}
	lines := strings.Split(strings.TrimRight(string(output), "\r\n"), "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "## ") {
		status.Branch = strings.TrimPrefix(lines[0], "## ")
		if before, _, ok := strings.Cut(status.Branch, "..."); ok {
			status.Branch = before
		}
		status.Branch = strings.TrimPrefix(status.Branch, "No commits yet on ")
		if status.Branch == "HEAD (no branch)" {
			status.Branch = ""
		}
	}
	for _, line := range lines[1:] {
		if len(line) < 2 {
			continue
		}
		code := line[:2]
		if code == "??" {
			status.Untracked++
			continue
		}
		if code[0] != ' ' {
			status.Staged++
		}
		if code[1] != ' ' {
			status.Unstaged++
		}
	}
	status.Clean = status.Staged == 0 && status.Unstaged == 0 && status.Untracked == 0
	return status, nil
}

func (e *CLIEngine) Branches(ctx context.Context, directory string) ([]Branch, error) {
	if !e.isWorktree(ctx, directory) {
		return []Branch{}, nil
	}
	output, err := e.run(ctx, directory, "for-each-ref", "--format=%(refname:short)%09%(objectname)%09%(HEAD)", "refs/heads")
	if err != nil {
		return nil, err
	}
	result := make([]Branch, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		current := len(fields) > 2 && strings.TrimSpace(fields[2]) == "*"
		result = append(result, Branch{Name: fields[0], CommitSHA: fields[1], Current: current})
	}
	return result, nil
}

func (e *CLIEngine) CreateBranch(ctx context.Context, directory, name, base string) error {
	if err := validateBranchName(ctx, e, name); err != nil {
		return err
	}
	if !e.isWorktree(ctx, directory) {
		return errors.New("initialize or clone a Git working copy first")
	}
	args := []string{"switch", "-c", name}
	if strings.TrimSpace(base) != "" {
		args = append(args, base)
	}
	_, err := e.run(ctx, directory, args...)
	return err
}

func (e *CLIEngine) SwitchBranch(ctx context.Context, directory, name string) error {
	if err := validateBranchName(ctx, e, name); err != nil {
		return err
	}
	if !e.isWorktree(ctx, directory) {
		return errors.New("initialize or clone a Git working copy first")
	}
	_, err := e.run(ctx, directory, "switch", name)
	return err
}

func safeRelativePath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsRune(raw, 0) || filepath.IsAbs(raw) {
		return "", errors.New("stage paths must be non-empty repository-relative paths")
	}
	clean := filepath.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("stage path must stay inside the repository")
	}
	return filepath.ToSlash(clean), nil
}

func (e *CLIEngine) Stage(ctx context.Context, directory string, paths []string) error {
	if !e.isWorktree(ctx, directory) {
		return errors.New("initialize or clone a Git working copy first")
	}
	args := []string{"add", "-A", "--"}
	if len(paths) == 0 {
		args = append(args, ".")
	} else {
		for _, raw := range paths {
			path, err := safeRelativePath(raw)
			if err != nil {
				return err
			}
			args = append(args, path)
		}
	}
	_, err := e.run(ctx, directory, args...)
	return err
}

func (e *CLIEngine) Commit(ctx context.Context, directory, message, authorName, authorEmail string) (Commit, error) {
	if !e.isWorktree(ctx, directory) {
		return Commit{}, errors.New("initialize or clone a Git working copy first")
	}
	message = strings.TrimSpace(message)
	if message == "" || len(message) > 10000 {
		return Commit{}, errors.New("commit message must contain between 1 and 10000 characters")
	}
	authorName, authorEmail = strings.TrimSpace(authorName), strings.TrimSpace(authorEmail)
	if (authorName == "") != (authorEmail == "") {
		return Commit{}, errors.New("author name and email must either both be provided or both be omitted")
	}
	args := []string{}
	if authorName != "" {
		args = append(args, "-c", "user.name="+authorName, "-c", "user.email="+authorEmail)
	}
	args = append(args, "commit", "--message", message)
	if _, err := e.run(ctx, directory, args...); err != nil {
		return Commit{}, err
	}
	items, err := e.Log(ctx, directory, 1)
	if err != nil {
		return Commit{}, err
	}
	if len(items) == 0 {
		return Commit{}, errors.New("Git commit completed but no commit could be read back")
	}
	return items[0], nil
}

func (e *CLIEngine) Log(ctx context.Context, directory string, limit int) ([]Commit, error) {
	if !e.isWorktree(ctx, directory) {
		return []Commit{}, nil
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	format := "%H%x1f%an%x1f%ae%x1f%aI%x1f%cn%x1f%ce%x1f%cI%x1f%s"
	output, err := e.run(ctx, directory, "log", "--no-color", "-n", fmt.Sprint(limit), "--format="+format)
	if err != nil {
		return nil, err
	}
	result := make([]Commit, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\x1f", 8)
		if len(fields) != 8 {
			continue
		}
		result = append(result, Commit{
			SHA: fields[0], Author: fields[1], AuthorEmail: fields[2], AuthorDate: fields[3],
			Committer: fields[4], CommitEmail: fields[5], CommitDate: fields[6], Message: fields[7],
		})
	}
	return result, nil
}

func validateTagName(ctx context.Context, e *CLIEngine, name string) error {
	raw := name
	name = strings.TrimSpace(name)
	if name == "" || raw != name || len(name) > 240 {
		return errors.New("a valid tag name is required")
	}
	if _, err := e.run(ctx, "", "check-ref-format", "refs/tags/"+name); err != nil {
		return fmt.Errorf("invalid tag name: %w", err)
	}
	return nil
}

func (e *CLIEngine) Tags(ctx context.Context, directory string) ([]Tag, error) {
	if !e.isWorktree(ctx, directory) {
		return []Tag{}, nil
	}
	format := "%(refname:short)%09%(objectname)%09%(objecttype)%09%(*objectname)%09%(contents:subject)"
	output, err := e.run(ctx, directory, "for-each-ref", "--format="+format, "refs/tags")
	if err != nil {
		return nil, err
	}
	result := make([]Tag, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 5)
		if len(fields) < 3 {
			continue
		}
		annotated := fields[2] == "tag"
		target := fields[1]
		message := ""
		if len(fields) > 3 && annotated && fields[3] != "" {
			target = fields[3]
		}
		if annotated && len(fields) > 4 {
			message = fields[4]
		}
		result = append(result, Tag{Name: fields[0], TargetSHA: target, Annotated: annotated, Message: message})
	}
	return result, nil
}

func (e *CLIEngine) CreateTag(ctx context.Context, directory, name, message string) error {
	if err := validateTagName(ctx, e, name); err != nil {
		return err
	}
	if !e.isWorktree(ctx, directory) {
		return errors.New("initialize or clone a Git working copy first")
	}
	message = strings.TrimSpace(message)
	if len(message) > 10000 {
		return errors.New("tag annotation must be 10000 characters or fewer")
	}
	if message == "" {
		_, err := e.run(ctx, directory, "tag", name)
		return err
	}

	// An annotated tag needs a tagger identity. Prefer this repository's configured
	// identity, then fall back to the author recorded on HEAD without mutating Git config.
	nameOutput, _ := e.run(ctx, directory, "config", "--get", "user.name")
	emailOutput, _ := e.run(ctx, directory, "config", "--get", "user.email")
	taggerName := strings.TrimSpace(string(nameOutput))
	taggerEmail := strings.TrimSpace(string(emailOutput))
	if taggerName == "" || taggerEmail == "" {
		headIdentity, identityErr := e.run(ctx, directory, "show", "-s", "--format=%an%x09%ae", "HEAD")
		if identityErr != nil {
			return errors.New("annotated tags require a commit with an author identity or a configured Git user.name and user.email")
		}
		fields := strings.SplitN(strings.TrimSpace(string(headIdentity)), "\t", 2)
		if len(fields) == 2 {
			if taggerName == "" {
				taggerName = strings.TrimSpace(fields[0])
			}
			if taggerEmail == "" {
				taggerEmail = strings.TrimSpace(fields[1])
			}
		}
	}
	if taggerName == "" || taggerEmail == "" {
		return errors.New("annotated tags require a Git tagger name and email")
	}
	_, err := e.run(ctx, directory, "-c", "user.name="+taggerName, "-c", "user.email="+taggerEmail, "tag", "--annotate", "--message", message, name)
	return err
}

func (e *CLIEngine) Diff(ctx context.Context, directory string, staged bool) (string, error) {
	if !e.isWorktree(ctx, directory) {
		return "", errors.New("initialize or clone a Git working copy first")
	}
	args := []string{"diff", "--no-ext-diff", "--no-color"}
	if staged {
		args = append(args, "--cached")
	}
	output, err := e.run(ctx, directory, args...)
	if err != nil {
		return "", err
	}
	if len(output) > maxDiffOutput {
		return "", errors.New("diff exceeded the 2 MiB display limit; narrow the staged paths")
	}
	return string(output), nil
}
