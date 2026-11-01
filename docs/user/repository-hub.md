# Connected Repository Hub

The Repository Hub contains provider-linked repositories. It is separate from IDIOS's original local/simulated repository records. Connecting a provider does not import every project automatically: search a connection, then explicitly link the repositories you choose.

## Before connecting an account

The IDIOS server must have a stable encryption key. Generate one once:

```sh
openssl rand -base64 32
```

Set the result in the environment of the process that runs `idios serve`:

```sh
export IDIOS_PROVIDER_ENCRYPTION_KEY='YOUR_PERSISTENT_BASE64_32_BYTE_KEY'
idios serve
```

On Windows PowerShell, first check whether you already have a key in your current session or User environment. Do **not** generate a replacement if a key already protects saved Hub credentials:

```powershell
$key = $env:IDIOS_PROVIDER_ENCRYPTION_KEY
if (-not $key) {
  $key = [Environment]::GetEnvironmentVariable('IDIOS_PROVIDER_ENCRYPTION_KEY', 'User')
}
if (-not $key) {
  $bytes = New-Object byte[] 32
  $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
  $rng.GetBytes($bytes)
  $key = [Convert]::ToBase64String($bytes)
  $rng.Dispose()
  [Environment]::SetEnvironmentVariable('IDIOS_PROVIDER_ENCRYPTION_KEY', $key, 'User')
}
$env:IDIOS_PROVIDER_ENCRYPTION_KEY = $key
$decoded = $null
try { $decoded = [Convert]::FromBase64String($key) } catch { }
if ((-not $decoded -or $decoded.Length -ne 32) -and $key -match '^[0-9a-fA-F]{64}
idios serve
```

This creates and saves a key only when neither the current process nor the Windows User environment has one. It also places the value in the current PowerShell process before starting IDIOS. If IDIOS is already running, stop that process first; the Hub service reads the key once when the Go server is initialized, so changing an environment variable after startup does not unlock the running process. Start the IDIOS server from this PowerShell session (or configure the same variable in the service manager that launches IDIOS).

Do not generate a new key every time IDIOS starts. Keep the same key in a protected service-manager secret or user environment. Back it up separately from the SQLite database. Losing or rotating the key makes saved provider credentials unreadable; reconnect those provider accounts after a key change. The key is never stored in the Web client or returned by the API.

If the key is absent or invalid, provider connections stay disabled and IDIOS displays the configuration requirement.

## GitHub OAuth

1. Create a GitHub OAuth App in your GitHub account's developer settings. This is currently an OAuth App flow, not GitHub App installation discovery.
2. Set its Authorization callback URL to `http://127.0.0.1:43170/api/hub/oauth/callback` when using the default local IDIOS address. If IDIOS uses a different loopback port, register that exact callback.
3. Set the OAuth App credentials in the server environment:

   ```sh
   export IDIOS_GITHUB_OAUTH_CLIENT_ID='...'
   export IDIOS_GITHUB_OAUTH_CLIENT_SECRET='...'
   ```

4. Restart IDIOS and select **Connected repositories → GitHub → Continue with GitHub OAuth**.

IDIOS requests the `repo` and `read:user` scopes. The access token, refresh token (when issued), token type, and expiry are stored together inside the AES-GCM encrypted credential envelope. When an OAuth access token approaches expiry, IDIOS exchanges its refresh token and persists the rotated token material before continuing. OAuth Apps that do not issue refresh tokens continue to work while their access token remains valid. GitHub App installation discovery and installation-token creation are not implemented yet. A personal access token can also be connected directly; manually supplied PATs must be rotated by the user.

## GitLab OAuth

1. Register an OAuth application on the GitLab instance you want to connect.
2. Register the same local callback URI: `http://127.0.0.1:43170/api/hub/oauth/callback` for the default IDIOS address.
3. Enable the `api` scope. IDIOS uses Authorization Code with PKCE (S256); a public client can omit its client secret.
4. Set the application ID (and optionally its secret) in the IDIOS server environment:

   ```sh
   export IDIOS_GITLAB_OAUTH_CLIENT_ID='...'
   export IDIOS_GITLAB_OAUTH_CLIENT_SECRET='...'
   ```

For a single GitLab OAuth application, configure `IDIOS_GITLAB_OAUTH_CLIENT_ID` and optionally `IDIOS_GITLAB_OAUTH_CLIENT_SECRET`. The instance must also be listed in `IDIOS_GITLAB_ALLOWED_HOSTS` unless it is GitLab.com. For multiple self-managed instances with different OAuth applications, use a JSON map keyed by normalized host origin:

```sh
export IDIOS_GITLAB_OAUTH_CLIENTS='{"https://gitlab.example.com":{"client_id":"instance-one-id","client_secret":"instance-one-secret"},"https://code.example.net":{"client_id":"instance-two-id","client_secret":"instance-two-secret"}}'
```

A matching host entry takes precedence over the single-instance fallback. Keep these values in the server's protected environment; do not commit them.

1. Restart IDIOS. Choose GitLab and the GitLab host, then select **Continue with GitLab OAuth**.

For self-managed GitLab, enter the instance origin only, for example `https://gitlab.example.com`. HTTPS is required. Because IDIOS makes server-side requests to this host, self-managed origins must also be explicitly trusted by the IDIOS operator. Add comma-separated origins to the server environment:

```sh
export IDIOS_GITLAB_ALLOWED_HOSTS='https://gitlab.example.com,https://code.example.net'
```

GitLab.com is allowed by default. Loopback is available only when `IDIOS_GITLAB_ALLOW_LOOPBACK=true` is explicitly set (for local development and integration tests only); do not enable it in production. The callback is restricted to the local IDIOS origin. The OAuth state is random, short-lived, stored as a hash, and consumed once. The access token, refresh token (when issued), token type, and expiry are stored in an encrypted envelope. IDIOS refreshes the access token before it expires and persists rotated refresh tokens.

## SourceForge / Allura

SourceForge Allura uses its own OAuth settings page to issue a Bearer token for the REST API. Create a token at [SourceForge OAuth settings](https://sourceforge.net/auth/oauth/), then choose SourceForge and paste the generated Bearer token into the IDIOS connection form. IDIOS does not put that token in a URL.

SourceForge capabilities differ from GitHub and GitLab:

| Capability | Current behavior |
|---|---|
| Discover Git tools associated with the account | Allura REST API |
| Tracker issues and ticket discussion | Allura Tracker and Discussion APIs when mounted |
| Pull/Merge Requests | Not provided by the Allura Tracker/Discussion API |
| Branches, tree, file and commit reads | HTTPS Git transport when Git is installed and a clone URL is available |
| Remote file writes / push | Not advertised through the Allura REST API |

SourceForge code browsing needs the `git` executable on the machine running IDIOS. By default, IDIOS uses the anonymous HTTPS clone URL returned by Allura. This is suitable for public repositories only. The Allura Bearer token is used for Tracker/REST calls and is deliberately not forwarded to Git transport.

For private SourceForge repositories, configure SSH transport in the environment of the IDIOS server before starting it:

```sh
export IDIOS_SOURCEFORGE_GIT_TRANSPORT=ssh
```

Load the SourceForge-authorized private key into the user's OpenSSH agent (`ssh-agent` / `ssh-add`) and ensure `git.code.sf.net` has a verified entry in that user's `known_hosts`. IDIOS sets `BatchMode=yes` and `StrictHostKeyChecking=yes`; it never accepts an unknown host key automatically or stores a private key. The SSH clone URL uses the connected SourceForge account name as the SSH username. This option enables read-only Branch, Tree, File and Commit browsing. IDIOS creates a temporary shallow clone for a read request and removes it when the request finishes. No SourceForge Git write or push is emulated or reported as successful.

## Capabilities and provider differences

| Provider | Code read/write | Branches / commits | Issues / comments | Change requests |
|---|---|---|---|---|
| GitHub | REST read; revision-checked file commits when account can push | REST | REST | Pull Requests, approve, merge |
| GitLab | REST read; revision-checked file commits when role permits | REST | REST | Merge Requests, approve, merge |
| SourceForge | Git read transport; no REST file push | Git read transport | Allura Tracker / Discussion | Unsupported |

The UI checks the capability list returned for each linked repository. Unsupported actions are not shown as supported. A provider can still reject a request due to branch protection, project-level permission, policy, rate limits, or repository state; IDIOS surfaces that response rather than bypassing it.

## Using the Hub

1. Open **Repositories → Connected repositories**.
1. Connect a provider account using OAuth (GitHub/GitLab when configured) or a token.
1. Search repositories for that connection.
1. Click **Link** beside only the repositories you want IDIOS to track.
1. Open a linked repository to browse its remote code and work items.
1. Use **Sync metadata** to refresh name, default branch, permission, capabilities, and open-item counts.
1. Create, comment on, edit, and close remote Issues where the selected provider exposes those operations. SourceForge Tracker fields include project-defined status, assignee, private visibility, discussion settings, and custom fields.
1. Create and manage Pull/Merge Requests on GitHub and GitLab. SourceForge Allura does not provide equivalent Merge Request actions.
1. Use **Local organization & preferences** for an alias, pin, collection membership, and a private local note.
1. Add typed relations between linked repositories where useful.

Collections, pins, aliases, and repository relations are local IDIOS metadata. Unlinking a repository removes the IDIOS link and its local preferences; it does not delete the remote repository or revoke the provider account connection. Disconnecting a provider removes the connection and all Hub links belonging to it.

## Data and security boundary

- Provider credentials are encrypted at rest with AES-256-GCM. The provider, host, and stable account ID are authenticated as associated data.
- Raw OAuth state is never stored; only its SHA-256 digest and PKCE verifier are persisted. States expire after ten minutes and are single-use.
- Provider API calls are made by the local Go server. The browser never receives a provider access token or OAuth client secret.
- Repository discovery is read-only. A repository becomes linked only after the user selects **Link**.
- Local IDIOS repositories and their simulated files, branches, commits, issues, and reviews remain separate from remote Hub repositories.
- Remote file updates on GitHub/GitLab require the revision returned by the previous read plus a commit message. A stale revision is rejected rather than overwritten.
- Remote Issue creation, comments, change-request creation, approval and merge act on the selected provider immediately.
- SourceForge code reads use temporary clone directories. Do not treat those files as IDIOS-local repository snapshots.
- Back up the encryption key and SQLite database together. If the key is unavailable, saved credentials cannot be decrypted.

## Troubleshooting

- **Credential storage is disabled:** configure `IDIOS_PROVIDER_ENCRYPTION_KEY` and restart IDIOS.
- **OAuth is not configured:** set the provider OAuth client ID/secret as shown above, then restart.
- **OAuth callback fails:** check the exact callback URL registered with the provider and ensure it matches the local IDIOS host and port.
- **Provider says unauthorized:** reconnect the account or check provider token scopes.
- **Permission denied:** the account cannot perform that operation, or the provider policy/branch protection blocks it.
- **Too many requests:** wait for the provider's rate limit to reset and retry.
- **SourceForge code browser is unavailable:** install Git on the IDIOS host and make sure the linked tool returned an HTTPS clone URL.
- **SourceForge cannot create a change request:** Allura Tracker/Discussion is not a Pull/Merge Request API; IDIOS reports this as unsupported.
) {
  $decoded = New-Object byte[] 32
  for ($i = 0; $i -lt 32; $i++) {
    $decoded[$i] = [Convert]::ToByte($key.Substring($i * 2, 2), 16)
  }
}
if (-not $decoded -or $decoded.Length -ne 32) {
  throw 'IDIOS_PROVIDER_ENCRYPTION_KEY must decode to exactly 32 bytes (base64 or 64-character hex).'
}
idios serve
```

This creates and saves a key only when neither the current process nor the Windows User environment has one. It also places the value in the current PowerShell process before starting IDIOS. If IDIOS is already running, stop that process first; the Hub service reads the key once when the Go server is initialized, so changing an environment variable after startup does not unlock the running process. Start the IDIOS server from this PowerShell session (or configure the same variable in the service manager that launches IDIOS).

Do not generate a new key every time IDIOS starts. Keep the same key in a protected service-manager secret or user environment. Back it up separately from the SQLite database. Losing or rotating the key makes saved provider credentials unreadable; reconnect those provider accounts after a key change. The key is never stored in the Web client or returned by the API.

If the key is absent or invalid, provider connections stay disabled and IDIOS displays the configuration requirement.

## GitHub OAuth

1. Create a GitHub OAuth App in your GitHub account's developer settings. This is currently an OAuth App flow, not GitHub App installation discovery.
2. Set its Authorization callback URL to `http://127.0.0.1:43170/api/hub/oauth/callback` when using the default local IDIOS address. If IDIOS uses a different loopback port, register that exact callback.
3. Set the OAuth App credentials in the server environment:

   ```sh
   export IDIOS_GITHUB_OAUTH_CLIENT_ID='...'
   export IDIOS_GITHUB_OAUTH_CLIENT_SECRET='...'
   ```

4. Restart IDIOS and select **Connected repositories → GitHub → Continue with GitHub OAuth**.

IDIOS requests the `repo` and `read:user` scopes. The access token, refresh token (when issued), token type, and expiry are stored together inside the AES-GCM encrypted credential envelope. When an OAuth access token approaches expiry, IDIOS exchanges its refresh token and persists the rotated token material before continuing. OAuth Apps that do not issue refresh tokens continue to work while their access token remains valid. GitHub App installation discovery and installation-token creation are not implemented yet. A personal access token can also be connected directly; manually supplied PATs must be rotated by the user.

## GitLab OAuth

1. Register an OAuth application on the GitLab instance you want to connect.
2. Register the same local callback URI: `http://127.0.0.1:43170/api/hub/oauth/callback` for the default IDIOS address.
3. Enable the `api` scope. IDIOS uses Authorization Code with PKCE (S256); a public client can omit its client secret.
4. Set the application ID (and optionally its secret) in the IDIOS server environment:

   ```sh
   export IDIOS_GITLAB_OAUTH_CLIENT_ID='...'
   export IDIOS_GITLAB_OAUTH_CLIENT_SECRET='...'
   ```

For a single GitLab OAuth application, configure `IDIOS_GITLAB_OAUTH_CLIENT_ID` and optionally `IDIOS_GITLAB_OAUTH_CLIENT_SECRET`. The instance must also be listed in `IDIOS_GITLAB_ALLOWED_HOSTS` unless it is GitLab.com. For multiple self-managed instances with different OAuth applications, use a JSON map keyed by normalized host origin:

```sh
export IDIOS_GITLAB_OAUTH_CLIENTS='{"https://gitlab.example.com":{"client_id":"instance-one-id","client_secret":"instance-one-secret"},"https://code.example.net":{"client_id":"instance-two-id","client_secret":"instance-two-secret"}}'
```

A matching host entry takes precedence over the single-instance fallback. Keep these values in the server's protected environment; do not commit them.

1. Restart IDIOS. Choose GitLab and the GitLab host, then select **Continue with GitLab OAuth**.

For self-managed GitLab, enter the instance origin only, for example `https://gitlab.example.com`. HTTPS is required. Because IDIOS makes server-side requests to this host, self-managed origins must also be explicitly trusted by the IDIOS operator. Add comma-separated origins to the server environment:

```sh
export IDIOS_GITLAB_ALLOWED_HOSTS='https://gitlab.example.com,https://code.example.net'
```

GitLab.com is allowed by default. Loopback is available only when `IDIOS_GITLAB_ALLOW_LOOPBACK=true` is explicitly set (for local development and integration tests only); do not enable it in production. The callback is restricted to the local IDIOS origin. The OAuth state is random, short-lived, stored as a hash, and consumed once. The access token, refresh token (when issued), token type, and expiry are stored in an encrypted envelope. IDIOS refreshes the access token before it expires and persists rotated refresh tokens.

## SourceForge / Allura

SourceForge Allura uses its own OAuth settings page to issue a Bearer token for the REST API. Create a token at [SourceForge OAuth settings](https://sourceforge.net/auth/oauth/), then choose SourceForge and paste the generated Bearer token into the IDIOS connection form. IDIOS does not put that token in a URL.

SourceForge capabilities differ from GitHub and GitLab:

| Capability | Current behavior |
|---|---|
| Discover Git tools associated with the account | Allura REST API |
| Tracker issues and ticket discussion | Allura Tracker and Discussion APIs when mounted |
| Pull/Merge Requests | Not provided by the Allura Tracker/Discussion API |
| Branches, tree, file and commit reads | HTTPS Git transport when Git is installed and a clone URL is available |
| Remote file writes / push | Not advertised through the Allura REST API |

SourceForge code browsing needs the `git` executable on the machine running IDIOS. By default, IDIOS uses the anonymous HTTPS clone URL returned by Allura. This is suitable for public repositories only. The Allura Bearer token is used for Tracker/REST calls and is deliberately not forwarded to Git transport.

For private SourceForge repositories, configure SSH transport in the environment of the IDIOS server before starting it:

```sh
export IDIOS_SOURCEFORGE_GIT_TRANSPORT=ssh
```

Load the SourceForge-authorized private key into the user's OpenSSH agent (`ssh-agent` / `ssh-add`) and ensure `git.code.sf.net` has a verified entry in that user's `known_hosts`. IDIOS sets `BatchMode=yes` and `StrictHostKeyChecking=yes`; it never accepts an unknown host key automatically or stores a private key. The SSH clone URL uses the connected SourceForge account name as the SSH username. This option enables read-only Branch, Tree, File and Commit browsing. IDIOS creates a temporary shallow clone for a read request and removes it when the request finishes. No SourceForge Git write or push is emulated or reported as successful.

## Capabilities and provider differences

| Provider | Code read/write | Branches / commits | Issues / comments | Change requests |
|---|---|---|---|---|
| GitHub | REST read; revision-checked file commits when account can push | REST | REST | Pull Requests, approve, merge |
| GitLab | REST read; revision-checked file commits when role permits | REST | REST | Merge Requests, approve, merge |
| SourceForge | Git read transport; no REST file push | Git read transport | Allura Tracker / Discussion | Unsupported |

The UI checks the capability list returned for each linked repository. Unsupported actions are not shown as supported. A provider can still reject a request due to branch protection, project-level permission, policy, rate limits, or repository state; IDIOS surfaces that response rather than bypassing it.

## Using the Hub

1. Open **Repositories → Connected repositories**.
1. Connect a provider account using OAuth (GitHub/GitLab when configured) or a token.
1. Search repositories for that connection.
1. Click **Link** beside only the repositories you want IDIOS to track.
1. Open a linked repository to browse its remote code and work items.
1. Use **Sync metadata** to refresh name, default branch, permission, capabilities, and open-item counts.
1. Create, comment on, edit, and close remote Issues where the selected provider exposes those operations. SourceForge Tracker fields include project-defined status, assignee, private visibility, discussion settings, and custom fields.
1. Create and manage Pull/Merge Requests on GitHub and GitLab. SourceForge Allura does not provide equivalent Merge Request actions.
1. Use **Local organization & preferences** for an alias, pin, collection membership, and a private local note.
1. Add typed relations between linked repositories where useful.

Collections, pins, aliases, and repository relations are local IDIOS metadata. Unlinking a repository removes the IDIOS link and its local preferences; it does not delete the remote repository or revoke the provider account connection. Disconnecting a provider removes the connection and all Hub links belonging to it.

## Data and security boundary

- Provider credentials are encrypted at rest with AES-256-GCM. The provider, host, and stable account ID are authenticated as associated data.
- Raw OAuth state is never stored; only its SHA-256 digest and PKCE verifier are persisted. States expire after ten minutes and are single-use.
- Provider API calls are made by the local Go server. The browser never receives a provider access token or OAuth client secret.
- Repository discovery is read-only. A repository becomes linked only after the user selects **Link**.
- Local IDIOS repositories and their simulated files, branches, commits, issues, and reviews remain separate from remote Hub repositories.
- Remote file updates on GitHub/GitLab require the revision returned by the previous read plus a commit message. A stale revision is rejected rather than overwritten.
- Remote Issue creation, comments, change-request creation, approval and merge act on the selected provider immediately.
- SourceForge code reads use temporary clone directories. Do not treat those files as IDIOS-local repository snapshots.
- Back up the encryption key and SQLite database together. If the key is unavailable, saved credentials cannot be decrypted.

## Troubleshooting

- **Credential storage is disabled:** configure `IDIOS_PROVIDER_ENCRYPTION_KEY` and restart IDIOS.
- **OAuth is not configured:** set the provider OAuth client ID/secret as shown above, then restart.
- **OAuth callback fails:** check the exact callback URL registered with the provider and ensure it matches the local IDIOS host and port.
- **Provider says unauthorized:** reconnect the account or check provider token scopes.
- **Permission denied:** the account cannot perform that operation, or the provider policy/branch protection blocks it.
- **Too many requests:** wait for the provider's rate limit to reset and retry.
- **SourceForge code browser is unavailable:** install Git on the IDIOS host and make sure the linked tool returned an HTTPS clone URL.
- **SourceForge cannot create a change request:** Allura Tracker/Discussion is not a Pull/Merge Request API; IDIOS reports this as unsupported.
