package repositoryhub

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"idios/internal/domain"
)

const maxResponseBytes = 8 << 20

type Account struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type RemoteRepository struct {
	Provider             domain.RepositoryProvider `json:"provider"`
	Host                 string                    `json:"host"`
	SSHAccountName       string                    `json:"-"`
	ExternalID           string                    `json:"external_id"`
	Namespace            string                    `json:"namespace"`
	Name                 string                    `json:"name"`
	FullPath             string                    `json:"full_path"`
	Description          string                    `json:"description"`
	WebURL               string                    `json:"web_url"`
	CloneURL             string                    `json:"clone_url,omitempty"`
	CloneSSHURL          string                    `json:"clone_ssh_url,omitempty"`
	DefaultBranch        string                    `json:"default_branch,omitempty"`
	Language             string                    `json:"language,omitempty"`
	Visibility           string                    `json:"visibility,omitempty"`
	RepositoryType       string                    `json:"repository_type"`
	Permission           string                    `json:"permission,omitempty"`
	Capabilities         []string                  `json:"capabilities"`
	OpenIssueCount       int                       `json:"open_issue_count"`
	OpenChangeCount      int                       `json:"open_change_request_count"`
	TrackerMountPoint    string                    `json:"tracker_mount_point,omitempty"`
	DiscussionMountPoint string                    `json:"discussion_mount_point,omitempty"`
}

type Branch struct {
	Name      string `json:"name"`
	CommitSHA string `json:"commit_sha"`
	Protected bool   `json:"protected"`
	Default   bool   `json:"default"`
}

type TreeEntry struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	SHA  string `json:"sha,omitempty"`
	URL  string `json:"url,omitempty"`
}

type TreePage struct {
	Items   []TreeEntry `json:"items"`
	Page    int         `json:"page"`
	HasMore bool        `json:"has_more"`
}

type File struct {
	Path         string `json:"path"`
	Content      string `json:"content"`
	SHA          string `json:"sha,omitempty"`
	LastCommitID string `json:"last_commit_id,omitempty"`
	Size         int64  `json:"size"`
	Encoding     string `json:"encoding"`
}

type CommitStats struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Total     int `json:"total"`
}

type CommitFile struct {
	Path         string `json:"path"`
	Status       string `json:"status"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	Patch        string `json:"patch,omitempty"`
	PreviousPath string `json:"previous_path,omitempty"`
}

type CommitCheck struct {
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Conclusion  string     `json:"conclusion,omitempty"`
	Description string     `json:"description,omitempty"`
	URL         string     `json:"url,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Commit struct {
	SHA                   string        `json:"sha"`
	Message               string        `json:"message"`
	Author                string        `json:"author"`
	Email                 string        `json:"email,omitempty"`
	AuthoredAt            *time.Time    `json:"authored_at,omitempty"`
	Committer             string        `json:"committer,omitempty"`
	CommitterEmail        string        `json:"committer_email,omitempty"`
	Committed             time.Time     `json:"committed_at"`
	URL                   string        `json:"url,omitempty"`
	SignatureStatus       string        `json:"signature_status,omitempty"`
	SignatureReason       string        `json:"signature_reason,omitempty"`
	SignatureType         string        `json:"signature_type,omitempty"`
	SignatureSource       string        `json:"signature_source,omitempty"`
	SignatureVerifiedAt   *time.Time    `json:"signature_verified_at,omitempty"`
	ChecksAvailable       bool          `json:"checks_available"`
	ChecksState           string        `json:"checks_state,omitempty"`
	Checks                []CommitCheck `json:"checks,omitempty"`
	Stats                 CommitStats   `json:"stats"`
	Files                 []CommitFile  `json:"files,omitempty"`
	Diff                  string        `json:"diff,omitempty"`
	DiffAvailable         bool          `json:"diff_available"`
	DiffUnavailableReason string        `json:"diff_unavailable_reason,omitempty"`
}

type FeatureAvailability struct {
	Available bool   `json:"available"`
	State     string `json:"state"`
	Reason    string `json:"reason,omitempty"`
}

type ReleaseAsset struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name"`
	Label           string `json:"label,omitempty"`
	Platform        string `json:"platform,omitempty"`
	PlatformSource  string `json:"platform_source,omitempty"`
	Size            *int64 `json:"size,omitempty"`
	ContentType     string `json:"content_type,omitempty"`
	DownloadCount   *int64 `json:"download_count,omitempty"`
	Digest          string `json:"digest,omitempty"`
	URL             string `json:"url,omitempty"`
}

type Release struct {
	ID              string         `json:"id,omitempty"`
	TagName         string         `json:"tag_name"`
	Name            string         `json:"name"`
	TargetCommitish string         `json:"target_commitish,omitempty"`
	Body            string         `json:"body,omitempty"`
	Draft           *bool          `json:"draft,omitempty"`
	Prerelease      *bool          `json:"prerelease,omitempty"`
	State           string         `json:"state"`
	CreatedAt       *time.Time     `json:"created_at,omitempty"`
	PublishedAt     *time.Time     `json:"published_at,omitempty"`
	URL             string         `json:"url,omitempty"`
	AssetsAvailable bool           `json:"assets_available"`
	Assets          []ReleaseAsset `json:"assets"`
}

type ReleasePage struct {
	Items        []Release          `json:"items"`
	Page         int                `json:"page"`
	HasMore      bool               `json:"has_more"`
	Availability FeatureAvailability `json:"availability"`
}

type Tag struct {
	Name             string     `json:"name"`
	Kind             string     `json:"kind"`
	ObjectSHA        string     `json:"object_sha,omitempty"`
	ObjectType       string     `json:"object_type,omitempty"`
	TargetSHA        string     `json:"target_sha,omitempty"`
	Annotation       string     `json:"annotation,omitempty"`
	CreatedAt        *time.Time `json:"created_at,omitempty"`
	CommitCreatedAt  *time.Time `json:"commit_created_at,omitempty"`
	Protected        *bool      `json:"protected,omitempty"`
	ReleaseName      string     `json:"release_name,omitempty"`
	ReleaseURL       string     `json:"release_url,omitempty"`
	URL              string     `json:"url,omitempty"`
}

type TagPage struct {
	Items        []Tag              `json:"items"`
	Page         int                `json:"page"`
	HasMore      bool               `json:"has_more"`
	Availability FeatureAvailability `json:"availability"`
}

type TagCompareCommit struct {
	SHA        string     `json:"sha"`
	Title      string     `json:"title,omitempty"`
	Message    string     `json:"message,omitempty"`
	Author     string     `json:"author,omitempty"`
	Committer  string     `json:"committer,omitempty"`
	CommittedAt *time.Time `json:"committed_at,omitempty"`
	URL        string     `json:"url,omitempty"`
}

type TagCompareFile struct {
	Path         string `json:"path"`
	PreviousPath string `json:"previous_path,omitempty"`
	Status       string `json:"status"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	Patch        string `json:"patch,omitempty"`
}

type TagComparison struct {
	From             string             `json:"from"`
	To               string             `json:"to"`
	State            string             `json:"state"`
	MergeBaseSHA     string             `json:"merge_base_sha,omitempty"`
	AheadBy          int                `json:"ahead_by"`
	BehindBy         int                `json:"behind_by"`
	TotalCommits     int                `json:"total_commits"`
	Commits          []TagCompareCommit `json:"commits"`
	Files            []TagCompareFile   `json:"files"`
	CommitsTruncated bool               `json:"commits_truncated"`
	FilesTruncated   bool               `json:"files_truncated"`
	CountsAvailable  bool                 `json:"counts_available"`
	Availability     FeatureAvailability `json:"availability"`
}

type IssueCreateOptions struct {
	Status             string
	Assignee           string
	Private            bool
	DiscussionDisabled bool
	CustomFields       map[string]string
}

type IssueUpdateOptions struct {
	Title              *string
	Body               *string
	State              string
	Assignee           *string
	Labels             *[]string
	Private            *bool
	DiscussionDisabled *bool
	CustomFields       map[string]string
}

type Issue struct {
	ID                 string         `json:"id"`
	Number             int            `json:"number"`
	Title              string         `json:"title"`
	Body               string         `json:"body"`
	State              string         `json:"state"`
	Author             string         `json:"author,omitempty"`
	Assignee           string         `json:"assignee,omitempty"`
	Labels             []string       `json:"labels,omitempty"`
	Private            bool           `json:"private,omitempty"`
	DiscussionDisabled bool           `json:"discussion_disabled,omitempty"`
	CustomFields       map[string]any `json:"custom_fields,omitempty"`
	URL                string         `json:"url,omitempty"`
	CreatedAt          time.Time      `json:"created_at,omitempty"`
	UpdatedAt          time.Time      `json:"updated_at,omitempty"`
}

type ChangeRequest struct {
	ID           string    `json:"id"`
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	State        string    `json:"state"`
	Draft        bool      `json:"draft"`
	SourceBranch string    `json:"source_branch"`
	TargetBranch string    `json:"target_branch"`
	Author       string    `json:"author,omitempty"`
	URL          string    `json:"url,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

type FileWrite struct {
	Path          string
	Branch        string
	Content       string
	CommitMessage string
	ExpectedSHA   string
	CreateOnly    bool
}

type Provider struct {
	kind domain.RepositoryProvider
	host string
	base string
	http *http.Client
}

func New(provider domain.RepositoryProvider, host string) (*Provider, error) {
	host = strings.TrimRight(strings.TrimSpace(host), "/")
	var base string
	if provider == domain.RepositoryProviderGitHub {
		if host != "https://github.com" && host != "github.com" {
			return nil, fmt.Errorf("%w: only github.com is supported for GitHub", domain.ErrInvalid)
		}
		host, base = "https://github.com", "https://api.github.com"
		_ = host
		return newProvider(provider, host, base), nil
	}
	if provider == domain.RepositoryProviderSourceForge {
		if host != "https://sourceforge.net" && host != "sourceforge.net" {
			return nil, fmt.Errorf("%w: SourceForge Allura is hosted at sourceforge.net", domain.ErrInvalid)
		}
		host, base = "https://sourceforge.net", "https://sourceforge.net/rest"
		return newProvider(provider, host, base), nil
	}
	if provider != domain.RepositoryProviderGitLab {
		return nil, fmt.Errorf("%w: unsupported provider %q", domain.ErrInvalid, provider)
	}
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	parsed, err := url.Parse(host)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, fmt.Errorf("%w: GitLab host must be an origin URL", domain.ErrInvalid)
	}
	if parsed.Scheme != "https" && !isLoopback(parsed.Hostname()) {
		return nil, fmt.Errorf("%w: self-managed GitLab must use HTTPS", domain.ErrInvalid)
	}
	if !GitLabHostAllowed(parsed.String()) {
		return nil, fmt.Errorf("%w: self-managed GitLab host must be gitlab.com or an origin listed in IDIOS_GITLAB_ALLOWED_HOSTS", domain.ErrInvalid)
	}
	host = strings.TrimRight(parsed.String(), "/")
	base = host + "/api/v4"
	return newProvider(provider, host, base), nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// GitLabHostAllowed restricts self-managed GitLab instances to origins explicitly
// trusted by the IDIOS operator. GitLab.com remains available without extra config.
// Loopback is available only for local development and integration tests.
func GitLabHostAllowed(raw string) bool {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" ||
		parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return false
	}
	if parsed.Scheme == "https" && strings.EqualFold(parsed.Hostname(), "gitlab.com") {
		return true
	}
	if parsed.Scheme == "http" || isLoopback(parsed.Hostname()) {
		return isLoopback(parsed.Hostname()) && strings.EqualFold(os.Getenv("IDIOS_GITLAB_ALLOW_LOOPBACK"), "true")
	}
	for _, configured := range strings.Split(os.Getenv("IDIOS_GITLAB_ALLOWED_HOSTS"), ",") {
		configured = strings.TrimSpace(configured)
		if configured == "" {
			continue
		}
		if !strings.Contains(configured, "://") {
			configured = "https://" + configured
		}
		allowed, parseErr := url.Parse(configured)
		if parseErr != nil || allowed.Host == "" || allowed.User != nil ||
			allowed.RawQuery != "" || allowed.Fragment != "" ||
			(allowed.Path != "" && allowed.Path != "/") {
			continue
		}
		if strings.EqualFold(allowed.Scheme, parsed.Scheme) && strings.EqualFold(allowed.Host, parsed.Host) {
			return true
		}
	}
	return false
}

func newProvider(kind domain.RepositoryProvider, host, base string) *Provider {
	client := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("provider redirected too many times")
			}
			if len(via) > 0 && !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
				return fmt.Errorf("provider redirect to a different host was blocked")
			}
			return nil
		},
	}
	return &Provider{kind: kind, host: host, base: base, http: client}
}

func (p *Provider) Kind() domain.RepositoryProvider { return p.kind }
func (p *Provider) Host() string                    { return p.host }

func (p *Provider) endpoint(path string, query url.Values) string {
	u := strings.TrimRight(p.base, "/") + "/" + strings.TrimLeft(path, "/")
	if strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "http://") {
		u = path
	}
	if len(query) != 0 {
		u += "?" + query.Encode()
	}
	return u
}

func (p *Provider) request(ctx context.Context, token, method, path string, query url.Values, body io.Reader, contentType string) ([]byte, int, error) {
	targetURL := p.endpoint(path, query)
	target, err := url.Parse(targetURL)
	if err != nil || target == nil || target.Host == "" || target.User != nil || target.Fragment != "" {
		return nil, 0, fmt.Errorf("%w: provider request URL is invalid", domain.ErrInvalid)
	}
	base, baseErr := url.Parse(p.base)
	host, hostErr := url.Parse(p.host)
	if baseErr != nil || hostErr != nil || base == nil || host == nil {
		return nil, 0, fmt.Errorf("%w: configured provider origin is invalid", domain.ErrInvalid)
	}
	// Absolute paths are used for OAuth endpoints, but authenticated requests must
	// remain on one of the two origins configured for this provider.
	if !strings.EqualFold(target.Host, base.Host) && !strings.EqualFold(target.Host, host.Host) {
		return nil, 0, fmt.Errorf("%w: request target is outside the configured provider origins", domain.ErrPermission)
	}
	// The configured provider origin is already validated by New. Permit HTTP
	// only for loopback origins used by local integration tests and development.
	if target.Scheme != "https" && !(target.Scheme == "http" && isLoopback(target.Hostname())) {
		return nil, 0, fmt.Errorf("%w: provider requests require HTTPS outside loopback", domain.ErrInvalid)
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "IDIOS-Repository-Hub")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if p.kind == domain.RepositoryProviderGitLab {
		req.Header.Set("PRIVATE-TOKEN", token)
	}
	if p.kind == domain.RepositoryProviderGitHub {
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
	response, err := p.http.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, 0, err
		}
		return nil, 0, fmt.Errorf("%w: provider request failed: %v", domain.ErrUnavailable, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("%w: could not read provider response", domain.ErrUnavailable)
	}
	if len(data) > maxResponseBytes {
		return nil, response.StatusCode, fmt.Errorf("%w: provider response exceeds 8 MiB", domain.ErrUnavailable)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		switch response.StatusCode {
		case http.StatusUnauthorized:
			return nil, response.StatusCode, fmt.Errorf("%w: provider rejected the saved credential; reconnect the account", domain.ErrUnavailable)
		case http.StatusForbidden:
			message := strings.ToLower(string(data))
			if response.Header.Get("X-RateLimit-Remaining") == "0" ||
				response.Header.Get("Retry-After") != "" ||
				strings.Contains(message, "rate limit exceeded") ||
				strings.Contains(message, "secondary rate limit") {
				retryAfter := strings.TrimSpace(response.Header.Get("Retry-After"))
				detail := "provider request limit reached; retry later"
				if retryAfter != "" {
					detail += " (Retry-After " + retryAfter + ")"
				}
				return nil, response.StatusCode, fmt.Errorf("%w: %s", domain.ErrRateLimited, detail)
			}
			return nil, response.StatusCode, fmt.Errorf("%w: provider denied this operation or the credential lacks permission", domain.ErrPermission)
		case http.StatusNotFound:
			return nil, response.StatusCode, fmt.Errorf("%w: provider resource was not found or is not visible to this account", domain.ErrNotFound)
		case http.StatusConflict, http.StatusPreconditionFailed, http.StatusUnprocessableEntity:
			return nil, response.StatusCode, fmt.Errorf("%w: provider rejected the change because the resource changed or the request conflicts with current state", domain.ErrConflict)
		case http.StatusTooManyRequests:
			return nil, response.StatusCode, fmt.Errorf("%w: provider request limit reached; retry later", domain.ErrRateLimited)
		default:
			if response.StatusCode >= 500 {
				return nil, response.StatusCode, fmt.Errorf("%w: provider returned HTTP %d", domain.ErrUnavailable, response.StatusCode)
			}
			return nil, response.StatusCode, fmt.Errorf("%w: provider returned HTTP %d", domain.ErrInvalid, response.StatusCode)
		}
	}
	return data, response.StatusCode, nil
}

func (p *Provider) json(ctx context.Context, token, method, path string, query url.Values, input, output any) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	data, _, err := p.request(ctx, token, method, path, query, body, "application/json")
	if err != nil {
		return err
	}
	if output != nil {
		if err := json.Unmarshal(data, output); err != nil {
			return fmt.Errorf("%w: provider returned an unexpected JSON response", domain.ErrUnavailable)
		}
	}
	return nil
}

func (p *Provider) multipartForm(ctx context.Context, token, method, path string, query url.Values, form url.Values, output any) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, values := range form {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return fmt.Errorf("encode provider form field %q: %w", key, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish provider multipart form: %w", err)
	}
	data, _, err := p.request(ctx, token, method, path, query, &body, writer.FormDataContentType())
	if err != nil {
		return err
	}
	if output != nil {
		if err := json.Unmarshal(data, output); err != nil {
			return fmt.Errorf("%w: provider returned an unexpected JSON response", domain.ErrUnavailable)
		}
	}
	return nil
}

func (p *Provider) form(ctx context.Context, token, method, path string, query url.Values, form url.Values, output any) error {
	data, _, err := p.request(ctx, token, method, path, query, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return err
	}
	if output != nil {
		if err := json.Unmarshal(data, output); err != nil {
			return fmt.Errorf("%w: provider returned an unexpected JSON response", domain.ErrUnavailable)
		}
	}
	return nil
}

type OAuthTokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (p *Provider) ExchangeAuthorizationCode(ctx context.Context, code, redirectURI, clientID, clientSecret, verifier string) (OAuthTokenResponse, error) {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(redirectURI) == "" || strings.TrimSpace(clientID) == "" {
		return OAuthTokenResponse{}, fmt.Errorf("%w: OAuth code, redirect URI, and client ID are required", domain.ErrInvalid)
	}
	form := url.Values{
		"client_id":    {clientID},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}
	path := ""
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		if strings.TrimSpace(clientSecret) == "" {
			return OAuthTokenResponse{}, fmt.Errorf("%w: IDIOS_GITHUB_OAUTH_CLIENT_SECRET is required for GitHub OAuth", domain.ErrUnavailable)
		}
		form.Set("client_secret", clientSecret)
		path = "https://github.com/login/oauth/access_token"
	case domain.RepositoryProviderGitLab:
		form.Set("grant_type", "authorization_code")
		if strings.TrimSpace(clientSecret) != "" {
			form.Set("client_secret", clientSecret)
		}
		if strings.TrimSpace(verifier) == "" {
			return OAuthTokenResponse{}, fmt.Errorf("%w: GitLab OAuth PKCE verifier is missing", domain.ErrInvalid)
		}
		form.Set("code_verifier", verifier)
		path = p.host + "/oauth/token"
	default:
		return OAuthTokenResponse{}, fmt.Errorf("%w: this provider does not use the OAuth 2.0 authorization-code flow", domain.ErrUnsupported)
	}
	var result OAuthTokenResponse
	if err := p.form(ctx, "", http.MethodPost, path, nil, form, &result); err != nil {
		return OAuthTokenResponse{}, err
	}
	if result.Error != "" || result.AccessToken == "" {
		detail := strings.TrimSpace(result.ErrorDescription)
		if detail == "" {
			detail = result.Error
		}
		return OAuthTokenResponse{}, fmt.Errorf("%w: provider did not issue an access token: %s", domain.ErrPermission, detail)
	}
	return result, nil
}

func (p *Provider) RefreshAuthorizationToken(ctx context.Context, refreshToken, clientID, clientSecret string) (OAuthTokenResponse, error) {
	if strings.TrimSpace(refreshToken) == "" || strings.TrimSpace(clientID) == "" {
		return OAuthTokenResponse{}, fmt.Errorf("%w: OAuth refresh token and client ID are required", domain.ErrInvalid)
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	}
	var path string
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		if strings.TrimSpace(clientSecret) == "" {
			return OAuthTokenResponse{}, fmt.Errorf("%w: GitHub OAuth client secret is required for refresh", domain.ErrUnavailable)
		}
		form.Set("client_secret", clientSecret)
		path = "https://github.com/login/oauth/access_token"
	case domain.RepositoryProviderGitLab:
		if strings.TrimSpace(clientSecret) != "" {
			form.Set("client_secret", clientSecret)
		}
		path = p.host + "/oauth/token"
	default:
		return OAuthTokenResponse{}, fmt.Errorf("%w: OAuth refresh is not configured for this provider", domain.ErrUnsupported)
	}
	var result OAuthTokenResponse
	if err := p.form(ctx, "", http.MethodPost, path, nil, form, &result); err != nil {
		return OAuthTokenResponse{}, err
	}
	if result.Error != "" || result.AccessToken == "" {
		detail := strings.TrimSpace(result.ErrorDescription)
		if detail == "" {
			detail = result.Error
		}
		return OAuthTokenResponse{}, fmt.Errorf("%w: provider rejected the OAuth refresh: %s", domain.ErrUnavailable, detail)
	}
	return result, nil
}

func (p *Provider) Account(ctx context.Context, token string) (Account, error) {
	var raw struct {
		ID       any    `json:"id"`
		Login    string `json:"login"`
		Username string `json:"username"`
		Name     string `json:"name"`
		Avatar   string `json:"avatar_url"`
	}
	path := "user"
	if p.kind == domain.RepositoryProviderSourceForge {
		path = p.host + "/user"
	}
	if err := p.json(ctx, token, http.MethodGet, path, nil, nil, &raw); err != nil {
		return Account{}, err
	}
	id := fmt.Sprint(raw.ID)
	username := raw.Login
	if username == "" {
		username = raw.Username
	}
	if id == "" || id == "<nil>" || username == "" {
		return Account{}, fmt.Errorf("%w: provider did not return a stable account identity", domain.ErrUnavailable)
	}
	name := raw.Name
	if name == "" {
		name = username
	}
	return Account{ID: id, Username: username, DisplayName: name, AvatarURL: raw.Avatar}, nil
}

func (p *Provider) ListRepositories(ctx context.Context, token, search string, page int) ([]RemoteRepository, error) {
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		return nil, fmt.Errorf("%w: page is out of range", domain.ErrInvalid)
	}
	query := url.Values{"per_page": {"100"}, "page": {strconv.Itoa(page)}}
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		query.Set("sort", "updated")
		query.Set("affiliation", "owner,collaborator,organization_member")
		var rows []struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			HTML        string `json:"html_url"`
			Clone       string `json:"clone_url"`
			SSH         string `json:"ssh_url"`
			Default     string `json:"default_branch"`
			Language    string `json:"language"`
			Visibility  string `json:"visibility"`
			OpenIssues  int    `json:"open_issues_count"`
			Private     bool   `json:"private"`
			Owner       struct {
				Login string `json:"login"`
			} `json:"owner"`
			Permissions struct {
				Admin    bool `json:"admin"`
				Maintain bool `json:"maintain"`
				Push     bool `json:"push"`
				Triage   bool `json:"triage"`
				Pull     bool `json:"pull"`
			} `json:"permissions"`
		}
		if err := p.json(ctx, token, http.MethodGet, "user/repos", query, nil, &rows); err != nil {
			return nil, err
		}
		out := make([]RemoteRepository, 0, len(rows))
		for _, r := range rows {
			parts := strings.SplitN(r.FullName, "/", 2)
			if len(parts) != 2 {
				continue
			}
			canWriteCode := r.Permissions.Admin || r.Permissions.Maintain || r.Permissions.Push
			canWriteIssues := canWriteCode || r.Permissions.Triage
			permission := "read"
			if r.Permissions.Admin {
				permission = "admin"
			} else if canWriteCode {
				permission = "write"
			} else if r.Permissions.Triage {
				permission = "triage"
			}
			caps := []string{"code.read", "branches.read", "commits.read", "issues.read", "changes.read", "releases.read", "tags.read", "tags.compare"}
			if canWriteIssues {
				caps = append(caps, "issues.write")
			}
			if canWriteCode {
				caps = append(caps, "code.write", "changes.write", "changes.merge")
			}
			out = append(out, RemoteRepository{Provider: p.kind, Host: p.host, ExternalID: strconv.FormatInt(r.ID, 10), Namespace: parts[0], Name: r.Name, FullPath: r.FullName,
				Description: r.Description, WebURL: r.HTML, CloneURL: r.Clone, CloneSSHURL: r.SSH, DefaultBranch: r.Default, Language: r.Language,
				Visibility: r.Visibility, RepositoryType: "git", Permission: permission, Capabilities: caps, OpenIssueCount: r.OpenIssues})
		}
		return filterRepositories(out, search), nil
	case domain.RepositoryProviderGitLab:
		query.Set("membership", "true")
		query.Set("order_by", "last_activity_at")
		if search != "" {
			query.Set("search", search)
		}
		var rows []struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			Path        string `json:"path"`
			FullPath    string `json:"path_with_namespace"`
			Description string `json:"description"`
			Web         string `json:"web_url"`
			SSH         string `json:"ssh_url_to_repo"`
			Clone       string `json:"http_url_to_repo"`
			Default     string `json:"default_branch"`
			Visibility  string `json:"visibility"`
			OpenIssues  int    `json:"open_issues_count"`
			Namespace   struct {
				FullPath string `json:"full_path"`
			} `json:"namespace"`
			Permissions struct {
				ProjectAccess struct {
					AccessLevel int `json:"access_level"`
				} `json:"project_access"`
				GroupAccess struct {
					AccessLevel int `json:"access_level"`
				} `json:"group_access"`
			} `json:"permissions"`
		}
		if err := p.json(ctx, token, http.MethodGet, "projects", query, nil, &rows); err != nil {
			return nil, err
		}
		out := make([]RemoteRepository, 0, len(rows))
		for _, r := range rows {
			full := r.FullPath
			if full == "" {
				full = r.Path
			}
			parts := strings.SplitN(full, "/", 2)
			if len(parts) != 2 {
				parts = []string{r.Namespace.FullPath, r.Path}
			}
			permission := "read"
			access := r.Permissions.ProjectAccess.AccessLevel
			if access == 0 {
				access = r.Permissions.GroupAccess.AccessLevel
			}
			if access >= 40 {
				permission = "admin"
			} else if access >= 30 {
				permission = "write"
			}
			caps := []string{"code.read", "branches.read", "commits.read", "issues.read", "changes.read", "releases.read", "tags.read", "tags.compare"}
			if access >= 20 {
				caps = append(caps, "issues.write")
			}
			if access >= 30 {
				caps = append(caps, "code.write", "changes.write", "changes.merge")
			}
			out = append(out, RemoteRepository{Provider: p.kind, Host: p.host, ExternalID: strconv.FormatInt(r.ID, 10), Namespace: parts[0], Name: r.Name, FullPath: full,
				Description: r.Description, WebURL: r.Web, CloneURL: r.Clone, CloneSSHURL: r.SSH, DefaultBranch: r.Default, Visibility: r.Visibility,
				RepositoryType: "git", Permission: permission, Capabilities: caps, OpenIssueCount: r.OpenIssues})
		}
		return out, nil
	case domain.RepositoryProviderSourceForge:
		query.Set("limit", "100")
		query.Set("page", strconv.Itoa((page-1)*100))
		if search != "" {
			query.Set("q", search)
		}
		var raw struct {
			Tools []struct {
				URL         string `json:"url"`
				APIURL      string `json:"api_url"`
				MountPoint  string `json:"mount_point"`
				MountLabel  string `json:"mount_label"`
				Name        string `json:"name"`
				ProjectName string `json:"project_name"`
				CloneAnon   string `json:"clone_url_https_anon"`
				Clone       string `json:"clone_url_ro"`
			} `json:"tools"`
		}
		if err := p.json(ctx, token, http.MethodGet, "auth/tools/git", query, nil, &raw); err != nil {
			return nil, err
		}
		trackerByProject := map[string]string{}
		var trackers struct {
			Tools []struct {
				ProjectName string `json:"project_name"`
				MountPoint  string `json:"mount_point"`
				Name        string `json:"name"`
			} `json:"tools"`
		}
		if err := p.json(ctx, token, http.MethodGet, "auth/tools/tracker", url.Values{"limit": {"100"}}, nil, &trackers); err == nil {
			for _, t := range trackers.Tools {
				trackerByProject[t.ProjectName] = t.MountPoint
			}
		}
		out := make([]RemoteRepository, 0, len(raw.Tools))
		for _, r := range raw.Tools {
			if r.ProjectName == "" || r.MountPoint == "" {
				continue
			}
			full := r.ProjectName + "/" + r.MountPoint
			clone := r.CloneAnon
			if clone == "" {
				clone = r.Clone
			}
			caps := []string{"code.clone", "code.read", "branches.read", "commits.read", "tags.read", "tags.compare"}
			if trackerByProject[r.ProjectName] != "" {
				caps = append(caps, "issues.read", "issues.write")
			}
			out = append(out, RemoteRepository{Provider: p.kind, Host: p.host, ExternalID: full, Namespace: r.ProjectName, Name: r.MountPoint, FullPath: full,
				Description: r.MountLabel, WebURL: r.URL, CloneURL: clone, RepositoryType: "git", Permission: "read", Capabilities: caps,
				TrackerMountPoint: trackerByProject[r.ProjectName]})
		}
		return filterRepositories(out, search), nil
	default:
		return nil, fmt.Errorf("%w: unsupported provider", domain.ErrUnsupported)
	}
}

func filterRepositories(items []RemoteRepository, search string) []RemoteRepository {
	search = strings.ToLower(strings.TrimSpace(search))
	if search == "" {
		return items
	}
	out := make([]RemoteRepository, 0)
	for _, r := range items {
		if strings.Contains(strings.ToLower(r.FullPath+" "+r.Description+" "+r.Language), search) {
			out = append(out, r)
		}
	}
	return out
}

func (p *Provider) GetRepository(ctx context.Context, token, externalID, fullPath string) (RemoteRepository, error) {
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		owner, repo := splitFullPath(fullPath)
		if owner == "" || repo == "" {
			return RemoteRepository{}, fmt.Errorf("%w: repository owner/name is required", domain.ErrInvalid)
		}
		var r struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			HTML        string `json:"html_url"`
			Clone       string `json:"clone_url"`
			SSH         string `json:"ssh_url"`
			Default     string `json:"default_branch"`
			Language    string `json:"language"`
			Visibility  string `json:"visibility"`
			OpenIssues  int    `json:"open_issues_count"`
			Owner       struct {
				Login string `json:"login"`
			} `json:"owner"`
			Permissions struct {
				Admin    bool `json:"admin"`
				Maintain bool `json:"maintain"`
				Push     bool `json:"push"`
				Triage   bool `json:"triage"`
			} `json:"permissions"`
		}
		if err := p.json(ctx, token, http.MethodGet, "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo), nil, nil, &r); err != nil {
			return RemoteRepository{}, err
		}
		canWriteCode := r.Permissions.Admin || r.Permissions.Maintain || r.Permissions.Push
		canWriteIssues := canWriteCode || r.Permissions.Triage
		permission := "read"
		if r.Permissions.Admin {
			permission = "admin"
		} else if canWriteCode {
			permission = "write"
		} else if r.Permissions.Triage {
			permission = "triage"
		}
		caps := []string{"code.read", "branches.read", "commits.read", "issues.read", "changes.read", "releases.read", "tags.read", "tags.compare"}
		if canWriteIssues {
			caps = append(caps, "issues.write")
		}
		if canWriteCode {
			caps = append(caps, "code.write", "changes.write", "changes.merge")
		}
		return RemoteRepository{Provider: p.kind, Host: p.host, ExternalID: strconv.FormatInt(r.ID, 10), Namespace: r.Owner.Login, Name: r.Name, FullPath: r.FullName, Description: r.Description, WebURL: r.HTML, CloneURL: r.Clone, CloneSSHURL: r.SSH, DefaultBranch: r.Default, Language: r.Language, Visibility: r.Visibility, RepositoryType: "git", Permission: permission, Capabilities: caps, OpenIssueCount: r.OpenIssues}, nil
	case domain.RepositoryProviderGitLab:
		id := externalID
		if id == "" {
			id = fullPath
		}
		var r struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			FullPath    string `json:"path_with_namespace"`
			Description string `json:"description"`
			Web         string `json:"web_url"`
			Clone       string `json:"http_url_to_repo"`
			SSH         string `json:"ssh_url_to_repo"`
			Default     string `json:"default_branch"`
			Visibility  string `json:"visibility"`
			OpenIssues  int    `json:"open_issues_count"`
			Namespace   struct {
				FullPath string `json:"full_path"`
			} `json:"namespace"`
			Permissions struct {
				ProjectAccess struct {
					AccessLevel int `json:"access_level"`
				} `json:"project_access"`
				GroupAccess struct {
					AccessLevel int `json:"access_level"`
				} `json:"group_access"`
			} `json:"permissions"`
		}
		if err := p.json(ctx, token, "GET", "projects/"+url.PathEscape(id), nil, nil, &r); err != nil {
			return RemoteRepository{}, err
		}
		parts := strings.SplitN(r.FullPath, "/", 2)
		namespace := r.Namespace.FullPath
		if namespace == "" && len(parts) > 1 {
			namespace = parts[0]
		}
		access := r.Permissions.ProjectAccess.AccessLevel
		if access == 0 {
			access = r.Permissions.GroupAccess.AccessLevel
		}
		permission := "read"
		if access >= 40 {
			permission = "admin"
		} else if access >= 30 {
			permission = "write"
		}
		capabilities := []string{"code.read", "branches.read", "commits.read", "issues.read", "changes.read", "releases.read", "tags.read", "tags.compare"}
		if access >= 20 {
			capabilities = append(capabilities, "issues.write")
		}
		if access >= 30 {
			capabilities = append(capabilities, "code.write", "changes.write", "changes.merge")
		}
		return RemoteRepository{Provider: p.kind, Host: p.host, ExternalID: strconv.FormatInt(r.ID, 10), Namespace: namespace, Name: r.Name, FullPath: r.FullPath, Description: r.Description, WebURL: r.Web, CloneURL: r.Clone, CloneSSHURL: r.SSH, DefaultBranch: r.Default, Visibility: r.Visibility, RepositoryType: "git", Permission: permission, Capabilities: capabilities, OpenIssueCount: r.OpenIssues}, nil
	case domain.RepositoryProviderSourceForge:
		parts := strings.SplitN(fullPath, "/", 2)
		if len(parts) != 2 {
			return RemoteRepository{}, fmt.Errorf("%w: SourceForge project/repository is required", domain.ErrInvalid)
		}
		all, err := p.ListRepositories(ctx, token, "", 1)
		if err != nil {
			return RemoteRepository{}, err
		}
		for _, r := range all {
			if r.FullPath == fullPath {
				return r, nil
			}
		}
		return RemoteRepository{}, fmt.Errorf("%w: SourceForge repository %s", domain.ErrNotFound, fullPath)
	default:
		return RemoteRepository{}, domain.ErrUnsupported
	}
}

func (p *Provider) ListBranches(ctx context.Context, token string, repo RemoteRepository) ([]Branch, error) {
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		owner, name := splitFullPath(repo.FullPath)
		var rows []struct {
			Name      string `json:"name"`
			Protected bool   `json:"protected"`
			Commit    struct {
				SHA string `json:"sha"`
			} `json:"commit"`
		}
		if err := p.json(ctx, token, "GET", "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/branches", url.Values{"per_page": {"100"}}, nil, &rows); err != nil {
			return nil, err
		}
		out := make([]Branch, 0, len(rows))
		for _, r := range rows {
			out = append(out, Branch{Name: r.Name, CommitSHA: r.Commit.SHA, Protected: r.Protected, Default: r.Name == repo.DefaultBranch})
		}
		return out, nil
	case domain.RepositoryProviderGitLab:
		var rows []struct {
			Name      string `json:"name"`
			Default   bool   `json:"default"`
			Protected bool   `json:"protected"`
			Commit    struct {
				ID string `json:"id"`
			} `json:"commit"`
		}
		if err := p.json(ctx, token, "GET", "projects/"+url.PathEscape(repo.ExternalID)+"/repository/branches", url.Values{"per_page": {"100"}}, nil, &rows); err != nil {
			return nil, err
		}
		out := make([]Branch, 0, len(rows))
		for _, r := range rows {
			out = append(out, Branch{Name: r.Name, CommitSHA: r.Commit.ID, Protected: r.Protected, Default: r.Default})
		}
		return out, nil
	case domain.RepositoryProviderSourceForge:
		return sourceForgeBranches(ctx, token, repo)
	default:
		return nil, fmt.Errorf("%w: branch API is not available for this provider", domain.ErrUnsupported)
	}
}

func (p *Provider) ListTree(ctx context.Context, token string, repo RemoteRepository, ref, path, treeSHA string, page int) (TreePage, error) {
	if ref == "" {
		ref = repo.DefaultBranch
	}
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path != "" && !safeGitPath(path) {
		return TreePage{}, fmt.Errorf("%w: safe repository directory path is required", domain.ErrInvalid)
	}
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		return TreePage{}, fmt.Errorf("%w: tree page is out of range", domain.ErrInvalid)
	}

	switch p.kind {
	case domain.RepositoryProviderGitHub:
		if page != 1 {
			return TreePage{}, fmt.Errorf("%w: GitHub Git tree objects are not page-addressable", domain.ErrInvalid)
		}
		treeRef := ref
		if treeSHA != "" {
			if !validTreeSHA(treeSHA) {
				return TreePage{}, fmt.Errorf("%w: invalid Git tree object ID", domain.ErrInvalid)
			}
			treeRef = treeSHA
		} else if path != "" {
			return TreePage{}, fmt.Errorf("%w: a Git tree object ID is required to expand this GitHub directory", domain.ErrInvalid)
		}
		owner, name := splitFullPath(repo.FullPath)
		var raw struct {
			Truncated bool `json:"truncated"`
			Tree      []struct {
				Path string `json:"path"`
				Type string `json:"type"`
				Size int64  `json:"size"`
				SHA  string `json:"sha"`
				URL  string `json:"url"`
			} `json:"tree"`
		}
		if err := p.json(ctx, token, "GET", "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/git/trees/"+url.PathEscape(treeRef), nil, nil, &raw); err != nil {
			return TreePage{}, err
		}
		if raw.Truncated {
			return TreePage{}, fmt.Errorf("%w: GitHub truncated this directory listing; open the repository with Git for the complete tree", domain.ErrUnavailable)
		}
		out := make([]TreeEntry, 0, len(raw.Tree))
		for _, entry := range raw.Tree {
			typ := entry.Type
			if typ != "tree" {
				typ = "blob"
			}
			out = append(out, TreeEntry{
				Path: joinTreePath(path, entry.Path), Name: lastPath(entry.Path),
				Type: typ, Size: entry.Size, SHA: entry.SHA, URL: entry.URL,
			})
		}
		return TreePage{Items: out, Page: 1}, nil

	case domain.RepositoryProviderGitLab:
		query := url.Values{
			"ref": {ref}, "per_page": {"100"}, "page": {strconv.Itoa(page)},
		}
		if path != "" {
			query.Set("path", path)
		}
		var rows []struct {
			Path string `json:"path"`
			Name string `json:"name"`
			Type string `json:"type"`
			Size int64  `json:"size"`
			ID   string `json:"id"`
		}
		if err := p.json(ctx, token, "GET", "projects/"+url.PathEscape(repo.ExternalID)+"/repository/tree", query, nil, &rows); err != nil {
			return TreePage{}, err
		}
		out := make([]TreeEntry, 0, len(rows))
		for _, entry := range rows {
			typ := "blob"
			if entry.Type == "tree" {
				typ = "tree"
			}
			entryPath := entry.Path
			if entryPath == "" {
				entryPath = entry.Name
			}
			out = append(out, TreeEntry{
				Path: joinTreePath(path, entryPath), Name: entry.Name,
				Type: typ, Size: entry.Size, SHA: entry.ID,
			})
		}
		return TreePage{Items: out, Page: page, HasMore: len(rows) == 100}, nil

	case domain.RepositoryProviderSourceForge:
		return sourceForgeTreePage(ctx, token, repo, ref, path, page)
	default:
		return TreePage{}, fmt.Errorf("%w: code tree API is not available for this provider", domain.ErrUnsupported)
	}
}

func validTreeSHA(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, char := range value {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func joinTreePath(parent, child string) string {
	parent = strings.Trim(parent, "/")
	child = strings.Trim(child, "/")
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	if child == parent || strings.HasPrefix(child, parent+"/") {
		return child
	}
	return parent + "/" + child
}

func (p *Provider) ReadFile(ctx context.Context, token string, repo RemoteRepository, path, ref string) (File, error) {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" || strings.Contains(path, "..") {
		return File{}, fmt.Errorf("%w: safe repository file path is required", domain.ErrInvalid)
	}
	if ref == "" {
		ref = repo.DefaultBranch
	}
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		owner, name := splitFullPath(repo.FullPath)
		var raw struct {
			Path     string `json:"path"`
			SHA      string `json:"sha"`
			Size     int64  `json:"size"`
			Encoding string `json:"encoding"`
			Content  string `json:"content"`
		}
		if err := p.json(ctx, token, "GET", "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/contents/"+escapePath(path), url.Values{"ref": {ref}}, nil, &raw); err != nil {
			return File{}, err
		}
		content, err := decodeProviderFile(raw.Encoding, raw.Content)
		if err != nil {
			return File{}, err
		}
		return File{Path: raw.Path, Content: content, SHA: raw.SHA, Size: raw.Size, Encoding: "utf-8"}, nil
	case domain.RepositoryProviderGitLab:
		var raw struct {
			FilePath   string `json:"file_path"`
			BlobID     string `json:"blob_id"`
			LastCommit string `json:"last_commit_id"`
			Size       int64  `json:"size"`
			Encoding   string `json:"encoding"`
			Content    string `json:"content"`
		}
		if err := p.json(ctx, token, "GET", "projects/"+url.PathEscape(repo.ExternalID)+"/repository/files/"+escapePath(path), url.Values{"ref": {ref}}, nil, &raw); err != nil {
			return File{}, err
		}
		content, err := decodeProviderFile(raw.Encoding, raw.Content)
		if err != nil {
			return File{}, err
		}
		return File{Path: raw.FilePath, Content: content, SHA: raw.BlobID, LastCommitID: raw.LastCommit, Size: raw.Size, Encoding: "utf-8"}, nil
	case domain.RepositoryProviderSourceForge:
		return sourceForgeFile(ctx, token, repo, path, ref)
	default:
		return File{}, fmt.Errorf("%w: file browser is not available for this provider", domain.ErrUnsupported)
	}
}

func decodeProviderFile(encoding, content string) (string, error) {
	if encoding == "" || encoding == "base64" {
		data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content, "\n", ""))
		if err != nil {
			return "", fmt.Errorf("%w: provider file content is not valid base64", domain.ErrUnavailable)
		}
		return string(data), nil
	}
	if encoding == "text" {
		return content, nil
	}
	return "", fmt.Errorf("%w: provider file encoding %q is not supported", domain.ErrUnsupported, encoding)
}

func (p *Provider) ListCommits(ctx context.Context, token string, repo RemoteRepository, ref string, page int) ([]Commit, error) {
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		return nil, fmt.Errorf("%w: page is out of range", domain.ErrInvalid)
	}
	if ref == "" {
		ref = repo.DefaultBranch
	}
	q := url.Values{"per_page": {"50"}, "page": {strconv.Itoa(page)}}
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		owner, name := splitFullPath(repo.FullPath)
		q.Set("sha", ref)
		var rows []struct {
			SHA    string `json:"sha"`
			HTML   string `json:"html_url"`
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Name  string    `json:"name"`
					Email string    `json:"email"`
					Date  time.Time `json:"date"`
				} `json:"author"`
				Committer struct {
					Name  string    `json:"name"`
					Email string    `json:"email"`
					Date  time.Time `json:"date"`
				} `json:"committer"`
				Verification *struct {
					Verified   bool       `json:"verified"`
					Reason     string     `json:"reason"`
					Signature  string     `json:"signature"`
					VerifiedAt *time.Time `json:"verified_at"`
				} `json:"verification"`
			} `json:"commit"`
		}
		if err := p.json(ctx, token, "GET", "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/commits", q, nil, &rows); err != nil {
			return nil, err
		}
		out := make([]Commit, 0, len(rows))
		for _, row := range rows {
			signatureStatus, signatureType, signatureReason, signatureVerifiedAt := mapGitHubVerification(row.Commit.Verification)
			out = append(out, Commit{
				SHA: row.SHA, Message: row.Commit.Message,
				Author: row.Commit.Author.Name, Email: row.Commit.Author.Email,
				AuthoredAt: optionalTime(row.Commit.Author.Date),
				Committer:  row.Commit.Committer.Name, CommitterEmail: row.Commit.Committer.Email,
				Committed: row.Commit.Committer.Date, URL: row.HTML,
				SignatureStatus: signatureStatus, SignatureType: signatureType,
				SignatureReason: signatureReason, SignatureSource: "provider",
				SignatureVerifiedAt: signatureVerifiedAt,
			})
		}
		return out, nil

	case domain.RepositoryProviderGitLab:
		q.Set("ref_name", ref)
		q.Set("with_stats", "true")
		var rows []struct {
			ID             string    `json:"id"`
			Title          string    `json:"title"`
			Message        string    `json:"message"`
			Web            string    `json:"web_url"`
			AuthorName     string    `json:"author_name"`
			AuthorEmail    string    `json:"author_email"`
			AuthoredDate   time.Time `json:"authored_date"`
			CommitterName  string    `json:"committer_name"`
			CommitterEmail string    `json:"committer_email"`
			CommittedDate  time.Time `json:"committed_date"`
			Stats          struct {
				Additions int `json:"additions"`
				Deletions int `json:"deletions"`
				Total     int `json:"total"`
			} `json:"stats"`
		}
		if err := p.json(ctx, token, "GET", "projects/"+url.PathEscape(repo.ExternalID)+"/repository/commits", q, nil, &rows); err != nil {
			return nil, err
		}
		out := make([]Commit, 0, len(rows))
		for _, row := range rows {
			message := row.Message
			if message == "" {
				message = row.Title
			}
			out = append(out, Commit{
				SHA: row.ID, Message: message,
				Author: row.AuthorName, Email: row.AuthorEmail,
				AuthoredAt: optionalTime(row.AuthoredDate),
				Committer:  row.CommitterName, CommitterEmail: row.CommitterEmail,
				Committed: row.CommittedDate, URL: row.Web,
				SignatureStatus: "unknown", SignatureSource: "provider",
				Stats: CommitStats{Additions: row.Stats.Additions, Deletions: row.Stats.Deletions, Total: row.Stats.Total},
			})
		}
		return out, nil

	case domain.RepositoryProviderSourceForge:
		return sourceForgeCommits(ctx, token, repo, ref, page)
	default:
		return nil, fmt.Errorf("%w: commits are not available for this provider", domain.ErrUnsupported)
	}
}

func (p *Provider) GetCommit(ctx context.Context, token string, repo RemoteRepository, ref, sha string) (Commit, error) {
	sha = strings.TrimSpace(sha)
	if !validCommitSHA(sha) {
		return Commit{}, fmt.Errorf("%w: a full commit SHA is required", domain.ErrInvalid)
	}
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		return p.getGitHubCommit(ctx, token, repo, sha)
	case domain.RepositoryProviderGitLab:
		return p.getGitLabCommit(ctx, token, repo, sha)
	case domain.RepositoryProviderSourceForge:
		return sourceForgeCommit(ctx, token, repo, ref, sha)
	default:
		return Commit{}, fmt.Errorf("%w: commit details are not available for this provider", domain.ErrUnsupported)
	}
}

func validCommitSHA(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, char := range value {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func mapGitHubVerification(verification *struct {
	Verified   bool       `json:"verified"`
	Reason     string     `json:"reason"`
	Signature  string     `json:"signature"`
	VerifiedAt *time.Time `json:"verified_at"`
}) (string, string, string, *time.Time) {
	if verification == nil {
		return "unknown", "", "GitHub did not return signature verification data.", nil
	}
	signatureType := detectCommitSignatureType(verification.Signature)
	if strings.EqualFold(verification.Reason, "unsigned") {
		return "unsigned", signatureType, verification.Reason, verification.VerifiedAt
	}
	if verification.Verified {
		return "verified", signatureType, verification.Reason, verification.VerifiedAt
	}
	if strings.TrimSpace(verification.Signature) == "" && strings.TrimSpace(verification.Reason) == "" {
		return "unknown", signatureType, "GitHub did not return a signature or verification reason.", verification.VerifiedAt
	}
	return "unverified", signatureType, verification.Reason, verification.VerifiedAt
}

func detectCommitSignatureType(signature string) string {
	switch {
	case strings.Contains(signature, "BEGIN PGP SIGNATURE"):
		return "GPG"
	case strings.Contains(signature, "BEGIN SSH SIGNATURE"):
		return "SSH"
	case strings.Contains(signature, "BEGIN SIGNED MESSAGE"),
		strings.Contains(signature, "BEGIN PKCS7"),
		strings.Contains(signature, "BEGIN CMS"):
		return "S/MIME"
	default:
		return ""
	}
}

func mapGitLabSignatureStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "verified":
		return "verified"
	case "unverified":
		return "unverified"
	case "unsigned":
		return "unsigned"
	case "unknown":
		return "unknown"
	case "":
		return "unknown"
	default:
		return "unknown"
	}
}

func countCommitDiffLines(diff string) (int, int) {
	additions, deletions := 0, 0
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions++
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deletions++
		}
	}
	return additions, deletions
}

func (p *Provider) getGitHubCommit(ctx context.Context, token string, repo RemoteRepository, sha string) (Commit, error) {
	owner, name := splitFullPath(repo.FullPath)
	var raw struct {
		SHA    string `json:"sha"`
		HTML   string `json:"html_url"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name  string    `json:"name"`
				Email string    `json:"email"`
				Date  time.Time `json:"date"`
			} `json:"author"`
			Committer struct {
				Name  string    `json:"name"`
				Email string    `json:"email"`
				Date  time.Time `json:"date"`
			} `json:"committer"`
			Verification *struct {
				Verified   bool       `json:"verified"`
				Reason     string     `json:"reason"`
				Signature  string     `json:"signature"`
				VerifiedAt *time.Time `json:"verified_at"`
			} `json:"verification"`
		} `json:"commit"`
		Stats struct {
			Additions int `json:"additions"`
			Deletions int `json:"deletions"`
			Total     int `json:"total"`
		} `json:"stats"`
		Files []struct {
			Filename         string `json:"filename"`
			Status           string `json:"status"`
			PreviousFilename string `json:"previous_filename"`
			Additions        int    `json:"additions"`
			Deletions        int    `json:"deletions"`
			Patch            string `json:"patch"`
		} `json:"files"`
	}
	if err := p.json(ctx, token, "GET", "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/commits/"+url.PathEscape(sha), nil, nil, &raw); err != nil {
		return Commit{}, err
	}
	signatureStatus, signatureType, signatureReason, signatureVerifiedAt := mapGitHubVerification(raw.Commit.Verification)
	item := Commit{
		SHA: raw.SHA, Message: raw.Commit.Message,
		Author: raw.Commit.Author.Name, Email: raw.Commit.Author.Email,
		AuthoredAt: optionalTime(raw.Commit.Author.Date),
		Committer:  raw.Commit.Committer.Name, CommitterEmail: raw.Commit.Committer.Email,
		Committed: raw.Commit.Committer.Date, URL: raw.HTML,
		SignatureStatus: signatureStatus, SignatureType: signatureType,
		SignatureReason: signatureReason, SignatureSource: "provider",
		SignatureVerifiedAt: signatureVerifiedAt,
		Stats:               CommitStats{Additions: raw.Stats.Additions, Deletions: raw.Stats.Deletions, Total: raw.Stats.Total},
		DiffAvailable:       true, ChecksState: "unavailable",
	}
	item.Files = make([]CommitFile, 0, len(raw.Files))
	for _, file := range raw.Files {
		item.Files = append(item.Files, CommitFile{
			Path: file.Filename, Status: file.Status, PreviousPath: file.PreviousFilename,
			Additions: file.Additions, Deletions: file.Deletions, Patch: file.Patch,
		})
	}

	var combined struct {
		State      string `json:"state"`
		TotalCount int    `json:"total_count"`
		Statuses   []struct {
			Context     string    `json:"context"`
			State       string    `json:"state"`
			Description string    `json:"description"`
			TargetURL   string    `json:"target_url"`
			CreatedAt   time.Time `json:"created_at"`
		} `json:"statuses"`
	}
	statusPath := "repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/commits/" + url.PathEscape(sha) + "/status"
	if err := p.json(ctx, token, "GET", statusPath, nil, nil, &combined); err == nil {
		item.ChecksAvailable = true
		item.ChecksState = combined.State
		for _, status := range combined.Statuses {
			item.Checks = append(item.Checks, CommitCheck{
				Name: status.Context, Status: status.State, Description: status.Description, URL: status.TargetURL,
				CompletedAt: optionalTime(status.CreatedAt),
			})
		}
	}

	var checkRuns struct {
		CheckRuns []struct {
			Name        string     `json:"name"`
			Status      string     `json:"status"`
			Conclusion  string     `json:"conclusion"`
			DetailsURL  string     `json:"details_url"`
			StartedAt   *time.Time `json:"started_at"`
			CompletedAt *time.Time `json:"completed_at"`
		} `json:"check_runs"`
	}
	checkPath := "repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/commits/" + url.PathEscape(sha) + "/check-runs"
	if err := p.json(ctx, token, "GET", checkPath, url.Values{"per_page": {"100"}}, nil, &checkRuns); err == nil {
		item.ChecksAvailable = true
		for _, check := range checkRuns.CheckRuns {
			item.Checks = append(item.Checks, CommitCheck{
				Name: check.Name, Status: check.Status, Conclusion: check.Conclusion, URL: check.DetailsURL,
				StartedAt: check.StartedAt, CompletedAt: check.CompletedAt,
			})
		}
	}
	if item.ChecksAvailable && len(item.Checks) == 0 && item.ChecksState == "" {
		item.ChecksState = "none"
	}
	return item, nil
}

func (p *Provider) getGitLabCommit(ctx context.Context, token string, repo RemoteRepository, sha string) (Commit, error) {
	var raw struct {
		ID             string    `json:"id"`
		Title          string    `json:"title"`
		Message        string    `json:"message"`
		WebURL         string    `json:"web_url"`
		AuthorName     string    `json:"author_name"`
		AuthorEmail    string    `json:"author_email"`
		AuthoredDate   time.Time `json:"authored_date"`
		CommitterName  string    `json:"committer_name"`
		CommitterEmail string    `json:"committer_email"`
		CommittedDate  time.Time `json:"committed_date"`
		Stats          struct {
			Additions int `json:"additions"`
			Deletions int `json:"deletions"`
			Total     int `json:"total"`
		} `json:"stats"`
		LastPipeline *struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
			WebURL string `json:"web_url"`
		} `json:"last_pipeline"`
	}
	base := "projects/" + url.PathEscape(repo.ExternalID) + "/repository/commits/" + url.PathEscape(sha)
	if err := p.json(ctx, token, "GET", base, url.Values{"stats": {"true"}}, nil, &raw); err != nil {
		return Commit{}, err
	}
	message := raw.Message
	if message == "" {
		message = raw.Title
	}
	item := Commit{
		SHA: raw.ID, Message: message,
		Author: raw.AuthorName, Email: raw.AuthorEmail, AuthoredAt: optionalTime(raw.AuthoredDate),
		Committer: raw.CommitterName, CommitterEmail: raw.CommitterEmail,
		Committed: raw.CommittedDate, URL: raw.WebURL,
		SignatureStatus: "unavailable", SignatureSource: "provider",
		Stats:       CommitStats{Additions: raw.Stats.Additions, Deletions: raw.Stats.Deletions, Total: raw.Stats.Total},
		ChecksState: "unavailable",
	}
	if raw.LastPipeline != nil {
		item.ChecksAvailable = true
		item.ChecksState = raw.LastPipeline.Status
		item.Checks = append(item.Checks, CommitCheck{
			Name: "Pipeline", Status: raw.LastPipeline.Status, URL: raw.LastPipeline.WebURL,
		})
	}

	var signature struct {
		SignatureType      string `json:"signature_type"`
		VerificationStatus string `json:"verification_status"`
		CommitSource       string `json:"commit_source"`
	}
	if err := p.json(ctx, token, "GET", base+"/signature", nil, nil, &signature); err == nil {
		item.SignatureStatus = mapGitLabSignatureStatus(signature.VerificationStatus)
		item.SignatureType = signature.SignatureType
		item.SignatureReason = signature.VerificationStatus
	} else if errors.Is(err, domain.ErrNotFound) {
		item.SignatureStatus = "unsigned"
		item.SignatureReason = "GitLab did not return a signature for this commit."
	}

	var statuses []struct {
		Name        string     `json:"name"`
		Status      string     `json:"status"`
		Description string     `json:"description"`
		TargetURL   string     `json:"target_url"`
		StartedAt   *time.Time `json:"started_at"`
		FinishedAt  *time.Time `json:"finished_at"`
	}
	if err := p.json(ctx, token, "GET", base+"/statuses", url.Values{"per_page": {"100"}}, nil, &statuses); err == nil {
		item.ChecksAvailable = true
		if raw.LastPipeline == nil && len(statuses) == 0 {
			item.ChecksState = "none"
		}
		for _, status := range statuses {
			item.Checks = append(item.Checks, CommitCheck{
				Name: status.Name, Status: status.Status, Description: status.Description, URL: status.TargetURL,
				StartedAt: status.StartedAt, CompletedAt: status.FinishedAt,
			})
		}
	}

	var diffs []struct {
		OldPath     string `json:"old_path"`
		NewPath     string `json:"new_path"`
		NewFile     bool   `json:"new_file"`
		RenamedFile bool   `json:"renamed_file"`
		DeletedFile bool   `json:"deleted_file"`
		Diff        string `json:"diff"`
	}
	if err := p.json(ctx, token, "GET", base+"/diff", url.Values{"per_page": {"100"}, "page": {"1"}}, nil, &diffs); err == nil {
		item.DiffAvailable = true
		for _, diff := range diffs {
			status := "modified"
			if diff.NewFile {
				status = "added"
			} else if diff.DeletedFile {
				status = "deleted"
			} else if diff.RenamedFile {
				status = "renamed"
			}
			additions, deletions := countCommitDiffLines(diff.Diff)
			item.Files = append(item.Files, CommitFile{
				Path: diff.NewPath, PreviousPath: diff.OldPath, Status: status,
				Additions: additions, Deletions: deletions, Patch: diff.Diff,
			})
		}
	}
	return item, nil
}
func (p *Provider) sourceForgeTicketIssue(ctx context.Context, token string, repo RemoteRepository, number int) (Issue, error) {
	if number < 1 || repo.TrackerMountPoint == "" {
		return Issue{}, fmt.Errorf("%w: SourceForge Tracker and positive ticket number are required", domain.ErrInvalid)
	}
	project, _ := splitFullPath(repo.FullPath)
	path := "p/" + url.PathEscape(project) + "/" + url.PathEscape(repo.TrackerMountPoint) + "/" + strconv.Itoa(number)
	var raw struct {
		Ticket struct {
			Summary            string         `json:"summary"`
			Description        string         `json:"description"`
			Status             string         `json:"status"`
			Reported           string         `json:"reported_by"`
			Assignee           string         `json:"assigned_to"`
			Private            bool           `json:"private"`
			DiscussionDisabled bool           `json:"discussion_disabled"`
			CustomFields       map[string]any `json:"custom_fields"`
			Created            time.Time      `json:"created_date"`
			Updated            time.Time      `json:"mod_date"`
			Labels             []string       `json:"labels"`
		} `json:"ticket"`
	}
	if err := p.json(ctx, token, http.MethodGet, path, nil, nil, &raw); err != nil {
		return Issue{}, err
	}
	t := raw.Ticket
	return Issue{
		ID: strconv.Itoa(number), Number: number, Title: t.Summary, Body: t.Description, State: t.Status,
		Author: t.Reported, Assignee: t.Assignee, Labels: t.Labels, Private: t.Private,
		DiscussionDisabled: t.DiscussionDisabled, CustomFields: t.CustomFields,
		URL: strings.TrimRight(p.host, "/") + "/" + path, CreatedAt: t.Created, UpdatedAt: t.Updated,
	}, nil
}

func (p *Provider) ListIssues(ctx context.Context, token string, repo RemoteRepository, state string, page int) ([]Issue, error) {
	if page < 1 {
		page = 1
	}
	q := url.Values{"per_page": {"100"}, "page": {strconv.Itoa(page)}}
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		owner, name := splitFullPath(repo.FullPath)
		if state == "" {
			state = "open"
		}
		q.Set("state", state)
		var rows []struct {
			Number  int       `json:"number"`
			Title   string    `json:"title"`
			Body    string    `json:"body"`
			State   string    `json:"state"`
			URL     string    `json:"html_url"`
			Created time.Time `json:"created_at"`
			Updated time.Time `json:"updated_at"`
			User    struct {
				Login string `json:"login"`
			} `json:"user"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
			Pull json.RawMessage `json:"pull_request"`
		}
		if err := p.json(ctx, token, "GET", "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/issues", q, nil, &rows); err != nil {
			return nil, err
		}
		out := make([]Issue, 0, len(rows))
		for _, r := range rows {
			if len(r.Pull) > 0 && string(r.Pull) != "null" {
				continue
			}
			labels := make([]string, 0, len(r.Labels))
			for _, l := range r.Labels {
				labels = append(labels, l.Name)
			}
			out = append(out, Issue{ID: strconv.Itoa(r.Number), Number: r.Number, Title: r.Title, Body: r.Body, State: r.State, Author: r.User.Login, Labels: labels, URL: r.URL, CreatedAt: r.Created, UpdatedAt: r.Updated})
		}
		return out, nil
	case domain.RepositoryProviderGitLab:
		if state == "" {
			state = "opened"
		}
		if state == "open" {
			state = "opened"
		}
		if state == "closed" {
			state = "closed"
		}
		q.Set("state", state)
		var rows []struct {
			IID         int       `json:"iid"`
			ID          int64     `json:"id"`
			Title       string    `json:"title"`
			Description string    `json:"description"`
			State       string    `json:"state"`
			URL         string    `json:"web_url"`
			Created     time.Time `json:"created_at"`
			Updated     time.Time `json:"updated_at"`
			Author      struct {
				Username string `json:"username"`
			} `json:"author"`
			Labels []string `json:"labels"`
		}
		if err := p.json(ctx, token, "GET", "projects/"+url.PathEscape(repo.ExternalID)+"/issues", q, nil, &rows); err != nil {
			return nil, err
		}
		out := make([]Issue, 0, len(rows))
		for _, r := range rows {
			out = append(out, Issue{ID: strconv.FormatInt(r.ID, 10), Number: r.IID, Title: r.Title, Body: r.Description, State: r.State, Author: r.Author.Username, Labels: r.Labels, URL: r.URL, CreatedAt: r.Created, UpdatedAt: r.Updated})
		}
		return out, nil
	case domain.RepositoryProviderSourceForge:
		if repo.TrackerMountPoint == "" {
			return nil, fmt.Errorf("%w: this SourceForge project has no linked Tracker tool", domain.ErrUnsupported)
		}
		project, _ := splitFullPath(repo.FullPath)
		alluraQuery := url.Values{
			"limit": {"100"},
			"page":  {strconv.Itoa((page - 1) * 100)},
		}
		var list struct {
			Tickets []struct {
				Number int `json:"ticket_num"`
			} `json:"tickets"`
		}
		if err := p.json(ctx, token, http.MethodGet, "p/"+url.PathEscape(project)+"/"+url.PathEscape(repo.TrackerMountPoint), alluraQuery, nil, &list); err != nil {
			return nil, err
		}
		out := make([]Issue, 0, len(list.Tickets))
		for _, listed := range list.Tickets {
			n := listed.Number
			if n < 1 {
				continue
			}
			var raw struct {
				Ticket struct {
					Summary            string         `json:"summary"`
					Description        string         `json:"description"`
					Status             string         `json:"status"`
					Reported           string         `json:"reported_by"`
					Assignee           string         `json:"assigned_to"`
					Private            bool           `json:"private"`
					DiscussionDisabled bool           `json:"discussion_disabled"`
					CustomFields       map[string]any `json:"custom_fields"`
					Created            time.Time      `json:"created_date"`
					Updated            time.Time      `json:"mod_date"`
					Labels             []string       `json:"labels"`
				} `json:"ticket"`
			}
			path := "p/" + url.PathEscape(project) + "/" + url.PathEscape(repo.TrackerMountPoint) + "/" + strconv.Itoa(n)
			if err := p.json(ctx, token, http.MethodGet, path, nil, nil, &raw); err != nil {
				return nil, err
			}
			t := raw.Ticket
			out = append(out, Issue{
				ID: strconv.Itoa(n), Number: n, Title: t.Summary, Body: t.Description, State: t.Status,
				Author: t.Reported, Assignee: t.Assignee, Labels: t.Labels, Private: t.Private,
				DiscussionDisabled: t.DiscussionDisabled, CustomFields: t.CustomFields,
				URL: strings.TrimRight(p.host, "/") + "/p/" + url.PathEscape(project) + "/" + url.PathEscape(repo.TrackerMountPoint) + "/" + strconv.Itoa(n), CreatedAt: t.Created, UpdatedAt: t.Updated,
			})
		}
		return out, nil
	default:
		return nil, domain.ErrUnsupported
	}
}

func (p *Provider) CreateIssue(ctx context.Context, token string, repo RemoteRepository, title, body string, labels []string) (Issue, error) {
	return p.CreateIssueWithOptions(ctx, token, repo, title, body, labels, IssueCreateOptions{})
}

func (p *Provider) CreateIssueWithOptions(ctx context.Context, token string, repo RemoteRepository, title, body string, labels []string, options IssueCreateOptions) (Issue, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 256 {
		return Issue{}, fmt.Errorf("%w: issue title is required and must be at most 256 characters", domain.ErrInvalid)
	}
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		if options.Private || options.DiscussionDisabled || options.Status != "" || options.Assignee != "" || len(options.CustomFields) > 0 {
			return Issue{}, fmt.Errorf("%w: GitHub Issues do not expose SourceForge Tracker privacy, status, or custom fields through this action", domain.ErrUnsupported)
		}
		owner, name := splitFullPath(repo.FullPath)
		var raw struct {
			Number  int       `json:"number"`
			Title   string    `json:"title"`
			Body    string    `json:"body"`
			State   string    `json:"state"`
			URL     string    `json:"html_url"`
			Created time.Time `json:"created_at"`
			Updated time.Time `json:"updated_at"`
		}
		if err := p.json(ctx, token, "POST", "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/issues", nil, map[string]any{"title": title, "body": body, "labels": labels}, &raw); err != nil {
			return Issue{}, err
		}
		return Issue{ID: strconv.Itoa(raw.Number), Number: raw.Number, Title: raw.Title, Body: raw.Body, State: raw.State, URL: raw.URL, CreatedAt: raw.Created, UpdatedAt: raw.Updated}, nil
	case domain.RepositoryProviderGitLab:
		if options.DiscussionDisabled || options.Status != "" || options.Assignee != "" || len(options.CustomFields) > 0 {
			return Issue{}, fmt.Errorf("%w: this GitLab Issue action does not support the requested Tracker fields", domain.ErrUnsupported)
		}
		var raw struct {
			IID          int       `json:"iid"`
			ID           int64     `json:"id"`
			Title        string    `json:"title"`
			Description  string    `json:"description"`
			State        string    `json:"state"`
			Confidential bool      `json:"confidential"`
			URL          string    `json:"web_url"`
			Created      time.Time `json:"created_at"`
			Updated      time.Time `json:"updated_at"`
		}
		payload := map[string]any{"title": title, "description": body, "labels": strings.Join(labels, ",")}
		if options.Private {
			payload["confidential"] = true
		}
		if err := p.json(ctx, token, http.MethodPost, "projects/"+url.PathEscape(repo.ExternalID)+"/issues", nil, payload, &raw); err != nil {
			return Issue{}, err
		}
		return Issue{ID: strconv.FormatInt(raw.ID, 10), Number: raw.IID, Title: raw.Title, Body: raw.Description, State: raw.State, Private: raw.Confidential, URL: raw.URL, CreatedAt: raw.Created, UpdatedAt: raw.Updated}, nil
	case domain.RepositoryProviderSourceForge:
		if repo.TrackerMountPoint == "" {
			return Issue{}, fmt.Errorf("%w: this project has no Tracker tool", domain.ErrUnsupported)
		}
		project, _ := splitFullPath(repo.FullPath)
		if len(options.CustomFields) > 30 {
			return Issue{}, fmt.Errorf("%w: SourceForge Tracker custom-field count exceeds 30", domain.ErrInvalid)
		}
		form := url.Values{"ticket_form.summary": {title}, "ticket_form.description": {body}}
		if options.Status != "" {
			form.Set("ticket_form.status", options.Status)
		}
		if options.Assignee != "" {
			form.Set("ticket_form.assigned_to", options.Assignee)
		}
		if options.Private {
			form.Set("ticket_form.private", "true")
		}
		if options.DiscussionDisabled {
			form.Set("ticket_form.discussion_disabled", "true")
		}
		if len(labels) > 0 {
			form.Set("ticket_form.labels", strings.Join(labels, ","))
		}
		for key, value := range options.CustomFields {
			if !strings.HasPrefix(key, "_") || strings.ContainsAny(key, ". /\\") || len(key) > 80 || len(value) > 2000 {
				return Issue{}, fmt.Errorf("%w: invalid SourceForge Tracker custom field %q", domain.ErrInvalid, key)
			}
			form.Set("ticket_form.custom_fields."+key, value)
		}
		var raw struct {
			Ticket struct {
				Summary     string `json:"summary"`
				Description string `json:"description"`
				Status      string `json:"status"`
				Private     bool   `json:"private"`
			} `json:"ticket"`
			TicketNumber int `json:"ticket_num"`
		}
		if err := p.multipartForm(ctx, token, http.MethodPost, "p/"+url.PathEscape(project)+"/"+url.PathEscape(repo.TrackerMountPoint)+"/new", nil, form, &raw); err != nil {
			return Issue{}, err
		}
		number := raw.TicketNumber
		if number < 1 {
			return Issue{}, fmt.Errorf("%w: Allura created the ticket but did not return its ticket number", domain.ErrUnavailable)
		}
		created, err := p.sourceForgeTicketIssue(ctx, token, repo, number)
		if err != nil {
			return Issue{}, fmt.Errorf("%w: SourceForge created ticket %d but its current state could not be read: %v", domain.ErrUnavailable, number, err)
		}
		return created, nil
	default:
		return Issue{}, domain.ErrUnsupported
	}
}

func (p *Provider) UpdateIssue(ctx context.Context, token string, repo RemoteRepository, number int, options IssueUpdateOptions) (Issue, error) {
	if number < 1 {
		return Issue{}, fmt.Errorf("%w: issue number must be positive", domain.ErrInvalid)
	}
	if options.Title == nil && options.Body == nil && options.State == "" && options.Assignee == nil &&
		options.Labels == nil && options.Private == nil && options.DiscussionDisabled == nil && len(options.CustomFields) == 0 {
		return Issue{}, fmt.Errorf("%w: at least one Issue field must be changed", domain.ErrInvalid)
	}
	if options.Title != nil && (strings.TrimSpace(*options.Title) == "" || len(*options.Title) > 256) {
		return Issue{}, fmt.Errorf("%w: issue title must contain 1 to 256 characters", domain.ErrInvalid)
	}
	if options.Body != nil && len(*options.Body) > 1<<20 {
		return Issue{}, fmt.Errorf("%w: issue description exceeds 1 MiB", domain.ErrInvalid)
	}
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		if options.Private != nil || options.DiscussionDisabled != nil || len(options.CustomFields) > 0 {
			return Issue{}, fmt.Errorf("%w: GitHub Issues do not expose Tracker privacy, discussion, or custom fields", domain.ErrUnsupported)
		}
		payload := map[string]any{}
		if options.Title != nil {
			payload["title"] = *options.Title
		}
		if options.Body != nil {
			payload["body"] = *options.Body
		}
		if options.State != "" {
			state := strings.ToLower(strings.TrimSpace(options.State))
			if state != "open" && state != "closed" {
				return Issue{}, fmt.Errorf("%w: GitHub Issue state must be open or closed", domain.ErrInvalid)
			}
			payload["state"] = state
		}
		if options.Assignee != nil {
			assignee := strings.TrimSpace(*options.Assignee)
			if assignee == "" {
				payload["assignees"] = []string{}
			} else {
				payload["assignees"] = []string{assignee}
			}
		}
		if options.Labels != nil {
			payload["labels"] = *options.Labels
		}
		owner, name := splitFullPath(repo.FullPath)
		var raw struct {
			Number  int       `json:"number"`
			Title   string    `json:"title"`
			Body    string    `json:"body"`
			State   string    `json:"state"`
			URL     string    `json:"html_url"`
			Created time.Time `json:"created_at"`
			Updated time.Time `json:"updated_at"`
			User    struct {
				Login string `json:"login"`
			} `json:"user"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
		}
		if err := p.json(ctx, token, http.MethodPatch, "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/issues/"+strconv.Itoa(number), nil, payload, &raw); err != nil {
			return Issue{}, err
		}
		labels := make([]string, 0, len(raw.Labels))
		for _, label := range raw.Labels {
			labels = append(labels, label.Name)
		}
		return Issue{ID: strconv.Itoa(raw.Number), Number: raw.Number, Title: raw.Title, Body: raw.Body, State: raw.State, Author: raw.User.Login, Labels: labels, URL: raw.URL, CreatedAt: raw.Created, UpdatedAt: raw.Updated}, nil

	case domain.RepositoryProviderGitLab:
		if options.Assignee != nil {
			return Issue{}, fmt.Errorf("%w: GitLab assigns Issues by numeric user ID; this form accepts usernames only", domain.ErrUnsupported)
		}
		if options.DiscussionDisabled != nil || len(options.CustomFields) > 0 {
			return Issue{}, fmt.Errorf("%w: the current GitLab Issue form does not expose these Tracker-specific fields", domain.ErrUnsupported)
		}
		payload := map[string]any{}
		if options.Title != nil {
			payload["title"] = *options.Title
		}
		if options.Body != nil {
			payload["description"] = *options.Body
		}
		if options.Labels != nil {
			payload["labels"] = strings.Join(*options.Labels, ",")
		}
		if options.Private != nil {
			payload["confidential"] = *options.Private
		}
		if options.State != "" {
			switch strings.ToLower(strings.TrimSpace(options.State)) {
			case "open", "opened":
				payload["state_event"] = "reopen"
			case "closed":
				payload["state_event"] = "close"
			default:
				return Issue{}, fmt.Errorf("%w: GitLab Issue state must be opened or closed", domain.ErrInvalid)
			}
		}
		var raw struct {
			IID          int       `json:"iid"`
			ID           int64     `json:"id"`
			Title        string    `json:"title"`
			Description  string    `json:"description"`
			State        string    `json:"state"`
			Confidential bool      `json:"confidential"`
			URL          string    `json:"web_url"`
			Created      time.Time `json:"created_at"`
			Updated      time.Time `json:"updated_at"`
			Author       struct {
				Username string `json:"username"`
			} `json:"author"`
			Labels []string `json:"labels"`
		}
		if err := p.json(ctx, token, http.MethodPut, "projects/"+url.PathEscape(repo.ExternalID)+"/issues/"+strconv.Itoa(number), nil, payload, &raw); err != nil {
			return Issue{}, err
		}
		return Issue{ID: strconv.FormatInt(raw.ID, 10), Number: raw.IID, Title: raw.Title, Body: raw.Description, State: raw.State, Author: raw.Author.Username, Labels: raw.Labels, Private: raw.Confidential, URL: raw.URL, CreatedAt: raw.Created, UpdatedAt: raw.Updated}, nil

	case domain.RepositoryProviderSourceForge:
		if repo.TrackerMountPoint == "" {
			return Issue{}, fmt.Errorf("%w: this SourceForge project has no Tracker tool", domain.ErrUnsupported)
		}
		if len(options.CustomFields) > 30 {
			return Issue{}, fmt.Errorf("%w: SourceForge Tracker custom-field count exceeds 30", domain.ErrInvalid)
		}
		project, _ := splitFullPath(repo.FullPath)
		form := url.Values{}
		if options.Title != nil {
			form.Set("ticket_form.summary", *options.Title)
		}
		if options.Body != nil {
			form.Set("ticket_form.description", *options.Body)
		}
		if options.State != "" {
			form.Set("ticket_form.status", options.State)
		}
		if options.Assignee != nil {
			form.Set("ticket_form.assigned_to", *options.Assignee)
		}
		if options.Labels != nil {
			form.Set("ticket_form.labels", strings.Join(*options.Labels, ","))
		}
		if options.Private != nil {
			form.Set("ticket_form.private", strconv.FormatBool(*options.Private))
		}
		if options.DiscussionDisabled != nil {
			form.Set("ticket_form.discussion_disabled", strconv.FormatBool(*options.DiscussionDisabled))
		}
		for key, value := range options.CustomFields {
			if !strings.HasPrefix(key, "_") || strings.ContainsAny(key, ". /\\") || len(key) > 80 || len(value) > 2000 {
				return Issue{}, fmt.Errorf("%w: invalid SourceForge Tracker custom field %q", domain.ErrInvalid, key)
			}
			form.Set("ticket_form.custom_fields."+key, value)
		}
		path := "p/" + url.PathEscape(project) + "/" + url.PathEscape(repo.TrackerMountPoint) + "/" + strconv.Itoa(number)
		if err := p.multipartForm(ctx, token, http.MethodPost, path+"/save", nil, form, nil); err != nil {
			return Issue{}, err
		}
		updated, err := p.sourceForgeTicketIssue(ctx, token, repo, number)
		if err != nil {
			return Issue{}, fmt.Errorf("%w: Tracker save was submitted but the updated ticket could not be verified: %v", domain.ErrUnavailable, err)
		}
		return updated, nil
	default:
		return Issue{}, domain.ErrUnsupported
	}
}

func (p *Provider) CloseIssue(ctx context.Context, token string, repo RemoteRepository, number int) error {
	if number < 1 {
		return fmt.Errorf("%w: issue number must be positive", domain.ErrInvalid)
	}
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		owner, name := splitFullPath(repo.FullPath)
		return p.json(ctx, token, http.MethodPatch, "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/issues/"+strconv.Itoa(number), nil, map[string]string{"state": "closed"}, nil)
	case domain.RepositoryProviderGitLab:
		return p.json(ctx, token, http.MethodPut, "projects/"+url.PathEscape(repo.ExternalID)+"/issues/"+strconv.Itoa(number), nil, map[string]string{"state_event": "close"}, nil)
	case domain.RepositoryProviderSourceForge:
		return fmt.Errorf("%w: close-ticket workflow requires the Tracker's project-specific status configuration", domain.ErrUnsupported)
	default:
		return domain.ErrUnsupported
	}
}

func (p *Provider) CommentIssue(ctx context.Context, token string, repo RemoteRepository, number int, text string) error {
	if strings.TrimSpace(text) == "" || len(text) > 20000 {
		return fmt.Errorf("%w: comment must contain 1 to 20000 characters", domain.ErrInvalid)
	}
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		owner, name := splitFullPath(repo.FullPath)
		return p.json(ctx, token, "POST", "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/issues/"+strconv.Itoa(number)+"/comments", nil, map[string]string{"body": text}, nil)
	case domain.RepositoryProviderGitLab:
		return p.json(ctx, token, "POST", "projects/"+url.PathEscape(repo.ExternalID)+"/issues/"+strconv.Itoa(number)+"/notes", nil, map[string]string{"body": text}, nil)
	case domain.RepositoryProviderSourceForge:
		if repo.TrackerMountPoint == "" {
			return domain.ErrUnsupported
		}
		project, _ := splitFullPath(repo.FullPath)
		var ticket struct {
			Ticket struct {
				DiscussionDisabled bool `json:"discussion_disabled"`
			} `json:"ticket"`
			DiscussionURL string `json:"discussion_thread_url"`
		}
		if err := p.json(ctx, token, http.MethodGet, "p/"+url.PathEscape(project)+"/"+url.PathEscape(repo.TrackerMountPoint)+"/"+strconv.Itoa(number), nil, nil, &ticket); err != nil {
			return err
		}
		if ticket.Ticket.DiscussionDisabled {
			return fmt.Errorf("%w: comments are disabled for this SourceForge ticket", domain.ErrUnsupported)
		}
		discussionURL, err := url.Parse(ticket.DiscussionURL)
		expectedHost, hostErr := url.Parse(p.host)
		if err != nil || hostErr != nil || discussionURL.Scheme != "https" && expectedHost.Scheme == "https" ||
			!strings.EqualFold(discussionURL.Hostname(), expectedHost.Hostname()) {
			return fmt.Errorf("%w: SourceForge ticket returned an invalid discussion URL", domain.ErrUnavailable)
		}
		segments := strings.Split(strings.Trim(discussionURL.Path, "/"), "/")
		threadID := ""
		for i := 0; i+1 < len(segments); i++ {
			if segments[i] == "_discuss" && i+2 < len(segments) && segments[i+1] == "thread" {
				threadID = segments[i+2]
				break
			}
		}
		if threadID == "" || strings.ContainsAny(threadID, "/\\") || threadID == "." || threadID == ".." {
			return fmt.Errorf("%w: SourceForge ticket does not expose a valid discussion thread", domain.ErrUnavailable)
		}
		form := url.Values{"text": {text}}
		return p.form(ctx, token, "POST", "p/"+url.PathEscape(project)+"/"+url.PathEscape(repo.TrackerMountPoint)+"/_discuss/thread/"+url.PathEscape(threadID)+"/new", nil, form, nil)
	default:
		return domain.ErrUnsupported
	}
}

func (p *Provider) ListChanges(ctx context.Context, token string, repo RemoteRepository, state string, page int) ([]ChangeRequest, error) {
	if p.kind == domain.RepositoryProviderSourceForge {
		return nil, fmt.Errorf("%w: SourceForge Allura Tracker/Discussion API does not expose merge requests", domain.ErrUnsupported)
	}
	if page < 1 {
		page = 1
	}
	q := url.Values{"per_page": {"100"}, "page": {strconv.Itoa(page)}}
	if p.kind == domain.RepositoryProviderGitHub {
		owner, name := splitFullPath(repo.FullPath)
		if state == "" {
			state = "open"
		}
		q.Set("state", state)
		var rows []struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			Body   string `json:"body"`
			State  string `json:"state"`
			Draft  bool   `json:"draft"`
			URL    string `json:"html_url"`
			Head   struct {
				Ref string `json:"ref"`
			} `json:"head"`
			Base struct {
				Ref string `json:"ref"`
			} `json:"base"`
			User struct {
				Login string `json:"login"`
			} `json:"user"`
			Created time.Time `json:"created_at"`
			Updated time.Time `json:"updated_at"`
		}
		if err := p.json(ctx, token, "GET", "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/pulls", q, nil, &rows); err != nil {
			return nil, err
		}
		out := make([]ChangeRequest, 0, len(rows))
		for _, r := range rows {
			out = append(out, ChangeRequest{ID: strconv.Itoa(r.Number), Number: r.Number, Title: r.Title, Body: r.Body, State: r.State, Draft: r.Draft, SourceBranch: r.Head.Ref, TargetBranch: r.Base.Ref, Author: r.User.Login, URL: r.URL, CreatedAt: r.Created, UpdatedAt: r.Updated})
		}
		return out, nil
	}
	if state == "" {
		state = "opened"
	}
	if state == "open" {
		state = "opened"
	}
	q.Set("state", state)
	var rows []struct {
		IID         int    `json:"iid"`
		ID          int64  `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		State       string `json:"state"`
		Draft       bool   `json:"draft"`
		URL         string `json:"web_url"`
		Source      string `json:"source_branch"`
		Target      string `json:"target_branch"`
		Author      struct {
			Username string `json:"username"`
		} `json:"author"`
		Created time.Time `json:"created_at"`
		Updated time.Time `json:"updated_at"`
	}
	if err := p.json(ctx, token, "GET", "projects/"+url.PathEscape(repo.ExternalID)+"/merge_requests", q, nil, &rows); err != nil {
		return nil, err
	}
	out := make([]ChangeRequest, 0, len(rows))
	for _, r := range rows {
		out = append(out, ChangeRequest{ID: strconv.FormatInt(r.ID, 10), Number: r.IID, Title: r.Title, Body: r.Description, State: r.State, Draft: r.Draft, SourceBranch: r.Source, TargetBranch: r.Target, Author: r.Author.Username, URL: r.URL, CreatedAt: r.Created, UpdatedAt: r.Updated})
	}
	return out, nil
}

func (p *Provider) CreateChange(ctx context.Context, token string, repo RemoteRepository, title, body, source, target string) (ChangeRequest, error) {
	if p.kind == domain.RepositoryProviderSourceForge {
		return ChangeRequest{}, domain.ErrUnsupported
	}
	title = strings.TrimSpace(title)
	if title == "" || source == "" || target == "" {
		return ChangeRequest{}, fmt.Errorf("%w: title, source branch, and target branch are required", domain.ErrInvalid)
	}
	if p.kind == domain.RepositoryProviderGitHub {
		owner, name := splitFullPath(repo.FullPath)
		var r struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			Body   string `json:"body"`
			State  string `json:"state"`
			HTML   string `json:"html_url"`
			Head   struct {
				Ref string `json:"ref"`
			} `json:"head"`
			Base struct {
				Ref string `json:"ref"`
			} `json:"base"`
			Created time.Time `json:"created_at"`
			Updated time.Time `json:"updated_at"`
		}
		err := p.json(ctx, token, "POST", "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/pulls", nil, map[string]string{"title": title, "body": body, "head": source, "base": target}, &r)
		if err != nil {
			return ChangeRequest{}, err
		}
		return ChangeRequest{ID: strconv.Itoa(r.Number), Number: r.Number, Title: r.Title, Body: r.Body, State: r.State, SourceBranch: r.Head.Ref, TargetBranch: r.Base.Ref, URL: r.HTML, CreatedAt: r.Created, UpdatedAt: r.Updated}, nil
	}
	var r struct {
		IID         int       `json:"iid"`
		ID          int64     `json:"id"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		State       string    `json:"state"`
		URL         string    `json:"web_url"`
		Source      string    `json:"source_branch"`
		Target      string    `json:"target_branch"`
		Created     time.Time `json:"created_at"`
		Updated     time.Time `json:"updated_at"`
	}
	err := p.json(ctx, token, "POST", "projects/"+url.PathEscape(repo.ExternalID)+"/merge_requests", nil, map[string]string{"title": title, "description": body, "source_branch": source, "target_branch": target}, &r)
	if err != nil {
		return ChangeRequest{}, err
	}
	return ChangeRequest{ID: strconv.FormatInt(r.ID, 10), Number: r.IID, Title: r.Title, Body: r.Description, State: r.State, SourceBranch: r.Source, TargetBranch: r.Target, URL: r.URL, CreatedAt: r.Created, UpdatedAt: r.Updated}, nil
}

func (p *Provider) CloseChange(ctx context.Context, token string, repo RemoteRepository, number int) error {
	if number < 1 {
		return fmt.Errorf("%w: change request number must be positive", domain.ErrInvalid)
	}
	switch p.kind {
	case domain.RepositoryProviderGitHub:
		owner, name := splitFullPath(repo.FullPath)
		return p.json(ctx, token, http.MethodPatch, "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/pulls/"+strconv.Itoa(number), nil, map[string]string{"state": "closed"}, nil)
	case domain.RepositoryProviderGitLab:
		return p.json(ctx, token, http.MethodPut, "projects/"+url.PathEscape(repo.ExternalID)+"/merge_requests/"+strconv.Itoa(number), nil, map[string]string{"state_event": "close"}, nil)
	case domain.RepositoryProviderSourceForge:
		return domain.ErrUnsupported
	default:
		return domain.ErrUnsupported
	}
}

func (p *Provider) ApproveChange(ctx context.Context, token string, repo RemoteRepository, number int) error {
	if p.kind == domain.RepositoryProviderSourceForge {
		return domain.ErrUnsupported
	}
	if p.kind == domain.RepositoryProviderGitHub {
		owner, name := splitFullPath(repo.FullPath)
		return p.json(ctx, token, "POST", "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/pulls/"+strconv.Itoa(number)+"/reviews", nil, map[string]string{"event": "APPROVE"}, nil)
	}
	return p.json(ctx, token, "POST", "projects/"+url.PathEscape(repo.ExternalID)+"/merge_requests/"+strconv.Itoa(number)+"/approve", nil, map[string]string{}, nil)
}

func (p *Provider) MergeChange(ctx context.Context, token string, repo RemoteRepository, number int) error {
	if p.kind == domain.RepositoryProviderSourceForge {
		return domain.ErrUnsupported
	}
	if p.kind == domain.RepositoryProviderGitHub {
		owner, name := splitFullPath(repo.FullPath)
		return p.json(ctx, token, "PUT", "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/pulls/"+strconv.Itoa(number)+"/merge", nil, map[string]string{"merge_method": "squash"}, nil)
	}
	return p.json(ctx, token, "PUT", "projects/"+url.PathEscape(repo.ExternalID)+"/merge_requests/"+strconv.Itoa(number)+"/merge", nil, map[string]string{"should_remove_source_branch": "false"}, nil)
}

// WriteFile applies an optimistic concurrency check. Updating requires the exact SHA / last
// commit ID shown by the read API. Creating is allowed only when CreateOnly=true and the file
// did not exist. SourceForge writes require Git transport and are not emulated through REST.
func (p *Provider) WriteFile(ctx context.Context, token string, repo RemoteRepository, input FileWrite) (File, error) {
	input.Path = strings.Trim(strings.TrimSpace(input.Path), "/")
	if input.Path == "" || strings.Contains(input.Path, "..") || input.Branch == "" || strings.TrimSpace(input.CommitMessage) == "" {
		return File{}, fmt.Errorf("%w: safe path, branch, and commit message are required", domain.ErrInvalid)
	}
	if p.kind == domain.RepositoryProviderSourceForge {
		return File{}, fmt.Errorf("%w: SourceForge repository file writes require a Git commit and push; Allura REST has no code-write endpoint", domain.ErrUnsupported)
	}
	existing, readErr := p.ReadFile(ctx, token, repo, input.Path, input.Branch)
	if input.CreateOnly {
		if readErr == nil {
			return File{}, fmt.Errorf("%w: file already exists; reload before editing", domain.ErrConflict)
		}
		if !errors.Is(readErr, domain.ErrNotFound) {
			return File{}, readErr
		}
	} else {
		if readErr != nil {
			return File{}, readErr
		}
		if input.ExpectedSHA == "" {
			return File{}, fmt.Errorf("%w: expected file revision is required", domain.ErrInvalid)
		}
		if p.kind == domain.RepositoryProviderGitHub && existing.SHA != input.ExpectedSHA {
			return File{}, fmt.Errorf("%w: file changed on the provider; reload before saving", domain.ErrConflict)
		}
		if p.kind == domain.RepositoryProviderGitLab && existing.LastCommitID != input.ExpectedSHA {
			return File{}, fmt.Errorf("%w: file changed on the provider; reload before saving", domain.ErrConflict)
		}
	}
	if p.kind == domain.RepositoryProviderGitHub {
		owner, name := splitFullPath(repo.FullPath)
		payload := map[string]any{"message": input.CommitMessage, "content": base64.StdEncoding.EncodeToString([]byte(input.Content)), "branch": input.Branch}
		if !input.CreateOnly {
			payload["sha"] = input.ExpectedSHA
		}
		var raw struct {
			Content struct {
				Path string `json:"path"`
				SHA  string `json:"sha"`
				Size int64  `json:"size"`
			} `json:"content"`
		}
		if err := p.json(ctx, token, "PUT", "repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/contents/"+escapePath(input.Path), nil, payload, &raw); err != nil {
			return File{}, err
		}
		return File{Path: raw.Content.Path, Content: input.Content, SHA: raw.Content.SHA, Size: raw.Content.Size, Encoding: "utf-8"}, nil
	}
	payload := map[string]any{"branch": input.Branch, "commit_message": input.CommitMessage, "content": input.Content, "encoding": "text"}
	if !input.CreateOnly {
		payload["last_commit_id"] = input.ExpectedSHA
	}
	var raw struct {
		FilePath   string `json:"file_path"`
		BlobID     string `json:"blob_id"`
		LastCommit string `json:"last_commit_id"`
		Size       int64  `json:"size"`
	}
	method := http.MethodPost
	if !input.CreateOnly {
		method = http.MethodPut
	}
	if err := p.json(ctx, token, method, "projects/"+url.PathEscape(repo.ExternalID)+"/repository/files/"+escapePath(input.Path), nil, payload, &raw); err != nil {
		return File{}, err
	}
	return File{Path: raw.FilePath, Content: input.Content, SHA: raw.BlobID, LastCommitID: raw.LastCommit, Size: raw.Size, Encoding: "utf-8"}, nil
}

func splitFullPath(full string) (string, string) {
	parts := strings.SplitN(strings.Trim(full, "/"), "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
func lastPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}
func escapePath(path string) string {
	parts := strings.Split(path, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
