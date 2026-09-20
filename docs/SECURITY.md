# Security policy

---

## Supported versions

| Version | Security fixes |
|---|---|
| 0.2.x | ✅ yes |

---

## Threat model

Idios is a local-first personal tool. Its default attack surface is small:

- **No server.** Idios runs as a CLI process owned by the user. There is
  no network listener, no daemon, no web endpoint.
- **No cloud.** The default config makes zero outbound network calls. The
  JEV backend and any LLM backend are strictly opt-in.
- **Single-user storage.** `data/state/state.json` is read and written by
  the process that invoked `idios`. File permissions are the user's
  responsibility.
- **Secrets.** API keys are read from environment variables (e.g.
  `IDIOS_JEV_API_KEY`), never stored in the state file or configs.

The primary risk vectors are:

1. **Dependency vulnerabilities** — pydantic, rich, tomli. These are
   maintained upstream; we track CVEs and pin minimums in `pyproject.toml`.
2. **State file integrity** — if `data/state/state.json` is writable by
   other users or processes, they can corrupt or read your knowledge base.
3. **Config injection** — if an untrusted `local.toml` is loaded via
   `IDIOS_CONFIG_DIR`, it can redirect storage, change the JEV endpoint,
   or enable a model backend.
4. **Opt-in network paths** — when a model backend or JEV is configured,
   the TLS certificate of the endpoint is trusted by the system's CA bundle.
   A compromised or malicious endpoint could receive your goals and queries.

---

## Reporting a vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Report via **GitHub's private advisory flow**:

1. Go to <https://github.com/m-mdy-m/idios/security/advisories/new>
2. Fill in the form. Include:
   - Affected version(s)
   - Reproduction steps (be specific)
   - Potential impact
   - Suggested fix, if you have one

You will receive an acknowledgement within **72 hours**.

If the issue is confirmed, a fix will be prepared and a CVE requested if
appropriate. You will be credited in the release notes unless you ask not
to be.

### What not to report

- Vulnerabilities in optional external services (JEV endpoint, LLM
  backends). Those are outside idios's control.
- Issues that require the attacker to already have write access to the
  running process or the state file.
- Issues in upstream dependencies (report those to the upstream project;
  link us to the upstream advisory).

---

## Security hardening tips

### Protect the state file

```bash
chmod 600 data/state/state.json
```

Idios does not set permissions itself — the file inherits your umask.

### Keep secrets in env vars, never in config

```bash
# good
export IDIOS_JEV_API_KEY="sk-..."
idios decide --goal "..."

# bad — don't put the key in configs/local.toml
```

### Verify dependencies after updates

```bash
pip install pip-audit
pip-audit
```

Or use `dependabot` — the `.github/dependabot.yml` in this repo enables
automatic Python dependency PRs.