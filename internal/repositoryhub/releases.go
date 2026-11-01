package repositoryhub

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"idios/internal/domain"
)

const defaultHubPageSize = 20
const maxHubPageSize = 50

func normalizeHubPage(page, pageSize int) (int, int, error) {
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		return 0, 0, fmt.Errorf("%w: page is out of range", domain.ErrInvalid)
	}
	if pageSize == 0 {
		pageSize = defaultHubPageSize
	}
	if pageSize < 1 || pageSize > maxHubPageSize {
		return 0, 0, fmt.Errorf("%w: page size must be between 1 and %d", domain.ErrInvalid, maxHubPageSize)
	}
	return page, pageSize, nil
}

func int64Pointer(value int64) *int64 { return &value }
func boolPointer(value bool) *bool    { return &value }

func releasePlatformFromName(name string) string {
	value := strings.ToLower(name)
	architecture := ""
	switch {
	case strings.Contains(value, "arm64"), strings.Contains(value, "aarch64"):
		architecture = "ARM64"
	case strings.Contains(value, "x86_64"), strings.Contains(value, "amd64"), strings.Contains(value, "x64"):
		architecture = "x86_64"
	case strings.Contains(value, "i386"), strings.Contains(value, "i686"), strings.Contains(value, "x86"):
		architecture = "x86"
	case strings.Contains(value, "universal"):
		architecture = "Universal"
	}
	switch {
	case strings.Contains(value, "windows"), strings.Contains(value, "win32"), strings.Contains(value, "win64"):
		if architecture != "" {
			return "Windows " + architecture
		}
		return "Windows"
	case strings.Contains(value, "linux"):
		if architecture != "" {
			return "Linux " + architecture
		}
		return "Linux"
	case strings.Contains(value, "darwin"), strings.Contains(value, "macos"), strings.Contains(value, "mac-os"), strings.Contains(value, "osx"):
		if architecture != "" {
			return "macOS " + architecture
		}
		return "macOS"
	case strings.Contains(value, "android"):
		if architecture != "" {
			return "Android " + architecture
		}
		return "Android"
	case strings.Contains(value, "ios"), strings.Contains(value, "iphone"), strings.Contains(value, "ipad"):
		if architecture != "" {
			return "iOS " + architecture
		}
		return "iOS"
	case strings.Contains(value, "web"), strings.Contains(value, "pwa"):
		return "Web"
	default:
		return ""
	}
}

func platformSource(name string) string {
	if releasePlatformFromName(name) == "" {
		return ""
	}
	return "filename"
}

func providerDownloadURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil ||
		(parsed.Scheme != "https" && parsed.Scheme != "http") {
		return ""
	}
	return parsed.String()
}

func validRemoteTagName(name string) bool {
	if name == "" || len(name) > 255 || strings.TrimSpace(name) != name {
		return false
	}
	for _, char := range name {
		if char < 0x20 || char == 0x7f {
			return false
		}
	}
	return true
}

// ListReleases returns one bounded page of provider releases. SourceForge Allura
// does not expose a release API, so that limitation is returned as data instead
// of being confused with an empty release history.
func (p *Provider) ListReleases(ctx context.Context, token string, repo RemoteRepository, page, pageSize int) (ReleasePage, error) {
	page, pageSize, err := normalizeHubPage(page, pageSize)
	if err != nil {
		return ReleasePage{}, err
	}
	result := ReleasePage{
		Items:        make([]Release, 0),
		Page:         page,
		Availability: FeatureAvailability{Available: true, State: "available"},
	}
	query := url.Values{"per_page": {strconv.Itoa(pageSize)}, "page": {strconv.Itoa(page)}}

	switch p.kind {
	case domain.RepositoryProviderGitHub:
		owner, name := splitFullPath(repo.FullPath)
		var rows []struct {
			ID              int64      `json:"id"`
			TagName         string     `json:"tag_name"`
			Name            string     `json:"name"`
			TargetCommitish string     `json:"target_commitish"`
			Body            string     `json:"body"`
			Draft           bool       `json:"draft"`
			Prerelease      bool       `json:"prerelease"`
			CreatedAt       *time.Time `json:"created_at"`
			PublishedAt     *time.Time `json:"published_at"`
			HTMLURL         string     `json:"html_url"`
			Assets          []struct {
				ID                 int64  `json:"id"`
				Name               string `json:"name"`
				Label              string `json:"label"`
				Size               int64  `json:"size"`
				ContentType        string `json:"content_type"`
				DownloadCount      int64  `json:"download_count"`
				Digest             string `json:"digest"`
				BrowserDownloadURL string `json:"browser_download_url"`
			} `json:"assets"`
		}
		endpoint := "repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/releases"
		if err := p.json(ctx, token, http.MethodGet, endpoint, query, nil, &rows); err != nil {
			return ReleasePage{}, err
		}
		for _, row := range rows {
			name := row.Name
			if name == "" {
				name = row.TagName
			}
			state := "published"
			if row.Draft {
				state = "draft"
			} else if row.Prerelease {
				state = "prerelease"
			}
			item := Release{
				ID: strconv.FormatInt(row.ID, 10), TagName: row.TagName, Name: name,
				TargetCommitish: row.TargetCommitish, Body: row.Body,
				Draft: boolPointer(row.Draft), Prerelease: boolPointer(row.Prerelease),
				State: state, CreatedAt: row.CreatedAt, PublishedAt: row.PublishedAt,
				URL: providerDownloadURL(row.HTMLURL), AssetsAvailable: true,
				Assets: make([]ReleaseAsset, 0, len(row.Assets)),
			}
			for _, asset := range row.Assets {
				item.Assets = append(item.Assets, ReleaseAsset{
					ID: strconv.FormatInt(asset.ID, 10), Name: asset.Name, Label: asset.Label,
					Platform: releasePlatformFromName(asset.Name), PlatformSource: platformSource(asset.Name),
					Size: int64Pointer(asset.Size), ContentType: asset.ContentType,
					DownloadCount: int64Pointer(asset.DownloadCount), Digest: asset.Digest,
					URL: providerDownloadURL(asset.BrowserDownloadURL),
				})
			}
			result.Items = append(result.Items, item)
		}
		sort.SliceStable(result.Items, func(i, j int) bool {
			left := result.Items[i].PublishedAt
			if left == nil {
				left = result.Items[i].CreatedAt
			}
			right := result.Items[j].PublishedAt
			if right == nil {
				right = result.Items[j].CreatedAt
			}
			if left == nil {
				return false
			}
			if right == nil {
				return true
			}
			return left.After(*right)
		})
		result.HasMore = len(rows) == pageSize
		return result, nil
	case domain.RepositoryProviderGitLab:
		var rows []struct {
			TagName         string     `json:"tag_name"`
			Name            string     `json:"name"`
			Description     string     `json:"description"`
			CreatedAt       *time.Time `json:"created_at"`
			ReleasedAt      *time.Time `json:"released_at"`
			UpcomingRelease bool       `json:"upcoming_release"`
			Assets          struct {
				Links []struct {
					ID              int64  `json:"id"`
					Name            string `json:"name"`
					URL             string `json:"url"`
					DirectAssetPath string `json:"direct_asset_path"`
					LinkType        string `json:"link_type"`
				} `json:"links"`
			} `json:"assets"`
		}
		endpoint := "projects/" + url.PathEscape(repo.ExternalID) + "/releases"
		query.Set("order_by", "released_at")
		query.Set("sort", "desc")
		if err := p.json(ctx, token, http.MethodGet, endpoint, query, nil, &rows); err != nil {
			return ReleasePage{}, err
		}
		for _, row := range rows {
			name := row.Name
			if name == "" {
				name = row.TagName
			}
			state := "published"
			if row.UpcomingRelease {
				state = "upcoming"
			}
			item := Release{
				TagName: row.TagName, Name: name, Body: row.Description, State: state,
				CreatedAt: row.CreatedAt, PublishedAt: row.ReleasedAt,
				URL:             providerDownloadURL(strings.TrimRight(repo.WebURL, "/") + "/-/releases/" + url.PathEscape(row.TagName)),
				AssetsAvailable: true, Assets: make([]ReleaseAsset, 0, len(row.Assets.Links)),
			}
			for _, asset := range row.Assets.Links {
				item.Assets = append(item.Assets, ReleaseAsset{
					ID: strconv.FormatInt(asset.ID, 10), Name: asset.Name, Label: asset.LinkType,
					Platform: releasePlatformFromName(asset.Name), PlatformSource: platformSource(asset.Name),
					URL: providerDownloadURL(asset.URL),
				})
			}
			result.Items = append(result.Items, item)
		}
		result.HasMore = len(rows) == pageSize
		return result, nil

	case domain.RepositoryProviderSourceForge:
		result.Availability = FeatureAvailability{
			Available: false, State: "unsupported",
			Reason: "SourceForge Allura does not expose a provider-backed Releases API. Git tags remain available separately.",
		}
		return result, nil
	default:
		return ReleasePage{}, fmt.Errorf("%w: releases are not available for this provider", domain.ErrUnsupported)
	}
}

// ListTags returns a bounded provider-backed tag page. It never uses the
// local simulated RepositoryTag store.
func (p *Provider) ListTags(ctx context.Context, token string, repo RemoteRepository, page, pageSize int, cursor string) (TagPage, error) {
	page, pageSize, err := normalizeHubPage(page, pageSize)
	if err != nil {
		return TagPage{}, err
	}
	result := TagPage{
		Items: make([]Tag, 0), Page: page,
		Availability: FeatureAvailability{Available: true, State: "available"},
	}
	query := url.Values{"per_page": {strconv.Itoa(pageSize)}, "page": {strconv.Itoa(page)}}

	switch p.kind {
	case domain.RepositoryProviderGitHub:
		if page > 1 && cursor == "" {
			return TagPage{}, fmt.Errorf("%w: GitHub tag pagination requires the cursor from the previous page", domain.ErrInvalid)
		}
		if len(cursor) > 2048 || strings.TrimSpace(cursor) != cursor || strings.ContainsAny(cursor, "\r\n") {
			return TagPage{}, fmt.Errorf("%w: GitHub tag cursor is invalid", domain.ErrInvalid)
		}
		owner, name := splitFullPath(repo.FullPath)
		var raw struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
			Data struct {
				Repository *struct {
					Refs struct {
						Nodes []struct {
							Name   string          `json:"name"`
							Target githubTagObject `json:"target"`
						} `json:"nodes"`
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"refs"`
				} `json:"repository"`
			} `json:"data"`
		}
		tagQuery := strings.Join([]string{
			"query($owner: String!, $name: String!, $first: Int!, $after: String) {",
			"  repository(owner: $owner, name: $name) {",
			"    refs(refPrefix: \"refs/tags/\", first: $first, after: $after, orderBy: {field: TAG_COMMIT_DATE, direction: DESC}) {",
			"      nodes {",
			"        name",
			"        target {",
			"          __typename oid",
			"          ... on Commit { committedDate }",
			"          ... on Tag {",
			"            message tagger { date }",
			"            target {",
			"              __typename oid",
			"              ... on Commit { committedDate }",
			"              ... on Tag {",
			"                message tagger { date }",
			"                target { __typename oid ... on Commit { committedDate } }",
			"              }",
			"            }",
			"          }",
			"        }",
			"      }",
			"      pageInfo { hasNextPage endCursor }",
			"    }",
			"  }",
			"}",
		}, "\n")
		var after any
		if cursor != "" {
			after = cursor
		}
		input := map[string]any{
			"query": tagQuery,
			"variables": map[string]any{
				"owner": owner, "name": name, "first": pageSize, "after": after,
			},
		}
		if err := p.json(ctx, token, http.MethodPost, strings.TrimRight(p.base, "/")+"/graphql", nil, input, &raw); err != nil {
			return TagPage{}, err
		}
		if len(raw.Errors) > 0 {
			messages := make([]string, 0, len(raw.Errors))
			for _, graphError := range raw.Errors {
				if strings.TrimSpace(graphError.Message) != "" {
					messages = append(messages, boundedText(graphError.Message, 240))
				}
			}
			return TagPage{}, fmt.Errorf("%w: GitHub could not list tags: %s", domain.ErrUnavailable, strings.Join(messages, "; "))
		}
		if raw.Data.Repository == nil {
			return TagPage{}, fmt.Errorf("%w: GitHub repository was not found or is not visible to this account", domain.ErrNotFound)
		}
		result.HasMore = raw.Data.Repository.Refs.PageInfo.HasNextPage
		if result.HasMore {
			result.NextCursor = raw.Data.Repository.Refs.PageInfo.EndCursor
		}
		releaseLinks := make(map[string]githubTagRelease)
		if len(raw.Data.Repository.Refs.Nodes) > 0 {
			var releases []struct {
				Name    string `json:"name"`
				TagName string `json:"tag_name"`
				HTMLURL string `json:"html_url"`
			}
			releaseQuery := url.Values{"per_page": {"100"}, "page": {"1"}}
			if err := p.json(ctx, token, http.MethodGet, "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/releases", releaseQuery, nil, &releases); err == nil {
				for _, release := range releases {
					releaseLinks[release.TagName] = githubTagRelease{
						Name: release.Name,
						URL:  providerDownloadURL(release.HTMLURL),
					}
				}
			}
		}
		for _, node := range raw.Data.Repository.Refs.Nodes {
			if !validRemoteTagName(node.Name) {
				return TagPage{}, fmt.Errorf("%w: GitHub returned an invalid Git tag name", domain.ErrUnavailable)
			}
			item, err := mapGitHubTagNode(repo, node.Name, node.Target)
			if err != nil {
				return TagPage{}, err
			}
			if release, ok := releaseLinks[node.Name]; ok {
				item.ReleaseName = release.Name
				if item.ReleaseName == "" {
					item.ReleaseName = node.Name
				}
				item.ReleaseURL = release.URL
			}
			result.Items = append(result.Items, item)
		}
		return result, nil
	case domain.RepositoryProviderGitLab:
		endpoint := "projects/" + url.PathEscape(repo.ExternalID) + "/repository/tags"
		query.Set("order_by", "updated")
		query.Set("sort", "desc")
		var rows []struct {
			Name      string     `json:"name"`
			Message   string     `json:"message"`
			Target    string     `json:"target"`
			CreatedAt *time.Time `json:"created_at"`
			Protected bool       `json:"protected"`
			Commit    struct {
				ID        string     `json:"id"`
				CreatedAt *time.Time `json:"created_at"`
			} `json:"commit"`
			Release *struct {
				TagName string `json:"tag_name"`
			} `json:"release"`
		}
		if err := p.json(ctx, token, http.MethodGet, endpoint, query, nil, &rows); err != nil {
			return TagPage{}, err
		}
		for _, row := range rows {
			kind := "lightweight"
			if row.Message != "" {
				kind = "annotated"
			}
			commitSHA := row.Commit.ID
			if commitSHA == "" {
				commitSHA = row.Target
			}
			item := Tag{
				Name: row.Name, Kind: kind, ObjectType: "commit",
				TargetSHA: commitSHA, Annotation: row.Message, CreatedAt: row.CreatedAt,
				CommitCreatedAt: row.Commit.CreatedAt, Protected: boolPointer(row.Protected),
				URL: providerDownloadURL(strings.TrimRight(repo.WebURL, "/") + "/-/tree/" + url.PathEscape(row.Name)),
			}
			if row.Release != nil && row.Release.TagName != "" {
				item.ReleaseName = row.Release.TagName
				item.ReleaseURL = providerDownloadURL(strings.TrimRight(repo.WebURL, "/") + "/-/releases/" + url.PathEscape(row.Release.TagName))
			}
			result.Items = append(result.Items, item)
		}
		result.HasMore = len(rows) == pageSize
		return result, nil

	case domain.RepositoryProviderSourceForge:
		return sourceForgeTags(ctx, token, repo, page, pageSize)
	default:
		return TagPage{}, fmt.Errorf("%w: tags are not available for this provider", domain.ErrUnsupported)
	}
}

type githubTagObject struct {
	TypeName      string          `json:"__typename"`
	OID           string          `json:"oid"`
	CommittedDate *time.Time      `json:"committedDate"`
	Message       string          `json:"message"`
	Tagger        struct {
		Date *time.Time `json:"date"`
	} `json:"tagger"`
	Target *githubTagObject `json:"target"`
}

func mapGitHubTagNode(repo RemoteRepository, name string, target githubTagObject) (Tag, error) {
	item := Tag{
		Name:       name,
		ObjectSHA:  target.OID,
		ObjectType: target.TypeName,
		URL:        providerDownloadURL(strings.TrimRight(repo.WebURL, "/") + "/tree/" + url.PathEscape(name)),
	}
	current := &target
	for depth := 0; current != nil; depth++ {
		if depth >= 5 {
			return Tag{}, fmt.Errorf("%w: Git tag object nesting exceeds the supported limit", domain.ErrUnavailable)
		}
		switch current.TypeName {
		case "Tag":
			if depth == 0 {
				item.Kind = "annotated"
				item.Annotation = current.Message
				item.CreatedAt = current.Tagger.Date
			}
			current = current.Target
		case "Commit":
			if item.Kind == "" {
				item.Kind = "lightweight"
				item.ObjectSHA = current.OID
			}
			item.TargetSHA = current.OID
			item.ObjectType = "commit"
			item.CommitCreatedAt = current.CommittedDate
			current = nil
		default:
			item.Kind = "unknown"
			if item.TargetSHA == "" {
				item.TargetSHA = current.OID
			}
			item.ObjectType = current.TypeName
			current = nil
		}
	}
	return item, nil
}

type githubTagRelease struct {
	Name string
	URL  string
}

func (p *Provider) CompareTags(ctx context.Context, token string, repo RemoteRepository, from, to string) (TagComparison, error) {
	if !validRemoteTagName(from) || !validRemoteTagName(to) {
		return TagComparison{}, fmt.Errorf("%w: two valid tag names are required", domain.ErrInvalid)
	}
	if from == to {
		return TagComparison{}, fmt.Errorf("%w: choose two different tags to compare", domain.ErrInvalid)
	}
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		return p.compareGitHubTags(ctx, token, repo, from, to)
	case domain.RepositoryProviderGitLab:
		return p.compareGitLabTags(ctx, token, repo, from, to)
	case domain.RepositoryProviderSourceForge:
		return sourceForgeCompareTags(ctx, token, repo, from, to)
	default:
		return TagComparison{}, fmt.Errorf("%w: tag comparison is not available for this provider", domain.ErrUnsupported)
	}
}

func (p *Provider) compareGitHubTags(ctx context.Context, token string, repo RemoteRepository, from, to string) (TagComparison, error) {
	owner, name := splitFullPath(repo.FullPath)
	endpoint := "repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) +
		"/compare/" + url.PathEscape(from) + "..." + url.PathEscape(to)
	query := url.Values{"per_page": {"100"}, "page": {"1"}}
	var raw struct {
		Status       string `json:"status"`
		AheadBy      int    `json:"ahead_by"`
		BehindBy     int    `json:"behind_by"`
		TotalCommits int    `json:"total_commits"`
		MergeBase    struct {
			SHA string `json:"sha"`
		} `json:"merge_base_commit"`
		Commits []struct {
			SHA    string `json:"sha"`
			HTML   string `json:"html_url"`
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Name string     `json:"name"`
					Date *time.Time `json:"date"`
				} `json:"author"`
				Committer struct {
					Name string     `json:"name"`
					Date *time.Time `json:"date"`
				} `json:"committer"`
			} `json:"commit"`
		} `json:"commits"`
		Files []struct {
			Filename         string `json:"filename"`
			PreviousFilename string `json:"previous_filename"`
			Status           string `json:"status"`
			Additions        int    `json:"additions"`
			Deletions        int    `json:"deletions"`
			Patch            string `json:"patch"`
		} `json:"files"`
	}
	if err := p.json(ctx, token, http.MethodGet, endpoint, query, nil, &raw); err != nil {
		return TagComparison{}, err
	}
	result := TagComparison{
		From: from, To: to, State: raw.Status, MergeBaseSHA: raw.MergeBase.SHA,
		CountsAvailable: true,
		AheadBy:         raw.AheadBy, BehindBy: raw.BehindBy, TotalCommits: raw.TotalCommits,
		Commits:      make([]TagCompareCommit, 0, len(raw.Commits)),
		Files:        make([]TagCompareFile, 0, len(raw.Files)),
		Availability: FeatureAvailability{Available: true, State: "available"},
	}
	for _, commit := range raw.Commits {
		title := strings.SplitN(commit.Commit.Message, "\n", 2)[0]
		result.Commits = append(result.Commits, TagCompareCommit{
			SHA: commit.SHA, Title: title, Message: commit.Commit.Message,
			Author: commit.Commit.Author.Name, Committer: commit.Commit.Committer.Name,
			CommittedAt: commit.Commit.Committer.Date, URL: providerDownloadURL(commit.HTML),
		})
	}
	for _, file := range raw.Files {
		result.Files = append(result.Files, TagCompareFile{
			Path: file.Filename, PreviousPath: file.PreviousFilename, Status: file.Status,
			Additions: file.Additions, Deletions: file.Deletions, Patch: file.Patch,
		})
	}
	result.CommitsTruncated = result.TotalCommits > len(result.Commits)
	result.FilesTruncated = len(result.Files) >= 300
	return result, nil
}

func (p *Provider) compareGitLabTags(ctx context.Context, token string, repo RemoteRepository, from, to string) (TagComparison, error) {
	endpoint := "projects/" + url.PathEscape(repo.ExternalID) + "/repository/compare"
	query := url.Values{"from": {from}, "to": {to}, "straight": {"false"}}
	var raw struct {
		CompareTimeout bool `json:"compare_timeout"`
		CompareSameRef bool `json:"compare_same_ref"`
		Commits        []struct {
			ID        string     `json:"id"`
			Title     string     `json:"title"`
			Message   string     `json:"message"`
			Author    string     `json:"author_name"`
			Committer string     `json:"committer_name"`
			Date      *time.Time `json:"committed_date"`
			WebURL    string     `json:"web_url"`
		} `json:"commits"`
		Diffs []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			NewFile     bool   `json:"new_file"`
			RenamedFile bool   `json:"renamed_file"`
			DeletedFile bool   `json:"deleted_file"`
			Collapsed   bool   `json:"collapsed"`
			TooLarge    bool   `json:"too_large"`
			Diff        string `json:"diff"`
		} `json:"diffs"`
	}
	if err := p.json(ctx, token, http.MethodGet, endpoint, query, nil, &raw); err != nil {
		return TagComparison{}, err
	}
	state := "compared"
	availability := FeatureAvailability{Available: true, State: "available"}
	if raw.CompareSameRef {
		state = "identical"
	}
	if raw.CompareTimeout {
		state = "partial"
		availability.State = "partial"
		availability.Reason = "GitLab timed out while computing this comparison; returned files or commits may be incomplete."
	}
	result := TagComparison{
		From: from, To: to, State: state, TotalCommits: len(raw.Commits),
		Commits:      make([]TagCompareCommit, 0, len(raw.Commits)),
		Files:        make([]TagCompareFile, 0, len(raw.Diffs)),
		Availability: availability,
	}
	for _, commit := range raw.Commits {
		result.Commits = append(result.Commits, TagCompareCommit{
			SHA: commit.ID, Title: commit.Title, Message: commit.Message,
			Author: commit.Author, Committer: commit.Committer,
			CommittedAt: commit.Date, URL: providerDownloadURL(commit.WebURL),
		})
	}
	for _, diff := range raw.Diffs {
		status := "modified"
		switch {
		case diff.NewFile:
			status = "added"
		case diff.DeletedFile:
			status = "deleted"
		case diff.RenamedFile:
			status = "renamed"
		}
		additions, deletions := countCommitDiffLines(diff.Diff)
		result.Files = append(result.Files, TagCompareFile{
			Path: diff.NewPath, PreviousPath: diff.OldPath, Status: status,
			Additions: additions, Deletions: deletions, Patch: diff.Diff,
		})
		if diff.Collapsed || diff.TooLarge {
			result.FilesTruncated = true
		}
	}
	return result, nil
}
