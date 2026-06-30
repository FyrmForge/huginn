# Huginn Deployment Plan — local stack (norsestack) in serverconfig

Status: DRAFT for review. Only the CI workflow in this repo is implemented.

Model: **local stack** `norsestack` in serverconfig (NOT remote stacks — remote
= one repo per stack, but norsestack will host huginn + muninn from two repos).
The combined compose lives in serverconfig; huginn repo only builds the image.

## How stackr deploys (verified in stackr source)

- `stackr <name> update` resolves env → `docker compose pull` → `docker compose up -d`.
  See `internal/stackcmd/stackcmd.go:467,479`.
- **Stackr never builds images.** It only pulls. So huginn must ship a
  pre-built image to a registry; the compose references it by tag.

## Two places that change

### 1. huginn repo (this repo) — DONE
- `.github/workflows/release.yml` — on tag `vX.Y.Z`: build `cmd/site/Dockerfile`,
  push `ghcr.io/fyrmforge/huginn:<tag>` + `:latest`, then POST stackr `/deploy`
  `{"stack":"norsestack","tag":"<tag>"}`.
- That's it. No compose here — it lives in serverconfig.

### 2. serverconfig repo — DONE (files written, secrets pending)
- `stacks/norsestack/docker-compose.yml` — local compose: `huginn`
  (`ghcr.io/fyrmforge/huginn:${NORSESTACK_IMAGE_TAG}`) + `postgres:18-alpine`
  shared db. Traefik labels route `huginn.${BASE_DOMAIN}`, healthcheck
  `/api/health`. Volumes on pools (SSD=postgres, HDD=uploads).
- `stacks/auth/authelia/configuration.yml` — added `huginn` OIDC client.
- `stacks/auth/docker-compose.yml` — injects `HUGINN_OIDC_CLIENT_SECRET_HASH`.
- `.stackr.yaml` — added `deploy.norsestack`.

**Server-side `.env` vars to set** (the `.env` in this checkout is not the deploy
one; stackr writes empty placeholders for any missing — fill the real values on
the server):
```
NORSESTACK_IMAGE_TAG=v0.1.0           # bumped automatically by the deploy callout
HUGINN_DB_PASSWORD=<secret>
HUGINN_OIDC_CLIENT_SECRET=<plaintext> # same secret, hashed form below
HUGINN_OIDC_CLIENT_SECRET_HASH=<authelia pbkdf2 hash of the above>
HUGINN_ADMIN_GROUP=<lldap/authelia group that maps to huginn admin>
```
- ghcr pull on host: `docker login ghcr.io` once (or make the package public) —
  stackr mounts `~/.docker/config.json` so it reuses host creds.

### Decided: container runs as ROOT
Reverted the non-root hardening so the bind-mounted uploads volume is writable
without an init step — matches every other stack in serverconfig.

### CI / GitHub secrets (huginn repo)
- `STACKR_DEPLOY_URL` = `https://stackr.vulpe.dev`, `STACKR_TOKEN` = stackr bearer.
- ghcr push uses built-in `GITHUB_TOKEN` (no extra secret).

## huginn-specific env the compose MUST set (from main.go)

| Var | Value | Why |
|-----|-------|-----|
| `DATABASE_URL` | `postgres://huginn:***@huginn_db:5432/huginn?sslmode=disable` | internal net |
| `BASE_URL` | `https://huginn.vulpe.dev` | **required** — `ParseBaseURL` fails fast; also drives OIDC redirect + cookie domain |
| `DEV_MODE` | unset/false | `CookieSecure = !DEV_MODE`; prod cookies must be Secure |
| `TRUSTED_PROXIES` | docker/traefik subnet CIDR | else X-Forwarded-For ignored → wrong client IP + rate-limit key |
| `STORAGE_PATH` | mounted volume path | uploads; **see non-root gotcha** |
| `PORT` | 8080 (default) | matches Traefik label + healthcheck |

Optional: `GOOGLE_CLIENT_*`, `OUTLOOK_CLIENT_*` (calendar import connectors).

Migrations run automatically on startup — no separate migration job.

## OIDC wiring (decided: huginn-own auth via Authelia OIDC, NO Authelia middleware)

CalDAV needs Basic-Auth passthrough, so huginn's Traefik router gets **no**
`authelia@file` middleware. Login goes through Authelia-as-OIDC-provider instead.

### huginn env (set in compose / .stackr-deployment.yaml)
```
OIDC_ISSUER=https://auth.vulpe.dev
OIDC_CLIENT_ID=huginn
OIDC_CLIENT_SECRET=<plaintext secret>     # Authelia stores its hash
OIDC_ADMIN_CLAIM=groups
OIDC_ADMIN_VALUE=<admin group, e.g. huginn-admins>
```
huginn's redirect URI is `BASE_URL + /auth/oidc/callback`
→ `https://huginn.vulpe.dev/auth/oidc/callback`.
huginn reads `email`, `name`, `picture`, and admin from the `groups` claim
(`claimContains` handles array claims).

### Authelia client (add to serverconfig `stacks/auth/authelia/configuration.yml`,
mirror the owncloud block):
```yaml
- client_id: huginn
  client_name: Huginn
  client_secret: '{{ env "HUGINN_OIDC_CLIENT_SECRET_HASH" }}'
  public: false
  authorization_policy: one_factor
  redirect_uris:
    - 'https://huginn.{{ env "BASE_DOMAIN" }}/auth/oidc/callback'
  scopes: [openid, profile, email, groups]   # groups needed for admin mapping
  response_types: [code]
  grant_types: [authorization_code, refresh_token]
  token_endpoint_auth_method: client_secret_basic
```
Generate the secret + hash with `authelia crypto hash generate pbkdf2`; put the
**hash** in serverconfig `.env` as `HUGINN_OIDC_CLIENT_SECRET_HASH`, the
**plaintext** in huginn's `OIDC_CLIENT_SECRET`.

Registration: with OIDC, users are provisioned on first login (OIDC upsert).
Leave `ALLOW_REGISTRATION` unset (password self-signup off). Password login stays
available for any seeded local accounts; the OIDC button appears when configured.

## Open gotchas

1. **Non-root + uploads volume.** The Dockerfile now runs as uid 10001 and owns
   `/uploads`. If a host volume is mounted at `STORAGE_PATH`, it's root-owned and
   the app can't write. Fix: init the volume dir ownership to 10001, or set
   `STORAGE_PATH` to the in-image `/uploads` and mount the volume there with
   correct ownership.
2. **Health path** is `/api/health`, not `/health`.
3. **ghcr auth on the host** — the server must be able to pull the package.

## Decisions

1. **Registry/CI** — DECIDED: ghcr.io + GitHub Actions on tag, then callout to
   stackr `/deploy`. Implemented in `.github/workflows/release.yml`.
   - Stack is **norsestack** (will later also host muninn, shared DB). Image
     stays `ghcr.io/fyrmforge/huginn`.
   - **Known limitation, deferred**: stackr's `/deploy` bumps a per-STACK tag
     (`NORSESTACK_IMAGE_TAG`), not per-image. Fine while norsestack only runs
     huginn. Before muninn joins, stackr needs a per-image deploy field; then
     update this workflow's payload. Tracked, not done now.
2. **Auth front** — DECIDED: huginn-own auth, no Authelia middleware (keeps CalDAV
   Basic Auth working).
3. **Login mode** — DECIDED: OIDC via Authelia (see OIDC wiring above).

### OIDC ID-token claims (verified)
huginn reads claims from the **ID token only** (never UserInfo). Authelia
**v4.38.19** includes `email`/`name`/`groups` in the ID token **by default**, so
huginn works as-is — verified the `claims_policies` key does NOT exist in 4.38
(adding it breaks config validation → all SSO down).
**On upgrade to Authelia 4.39+**, ID tokens go minimal by default; THEN add:
```yaml
identity_providers:
  oidc:
    claims_policies:
      huginn:
        id_token: ['groups','email','email_verified','name','preferred_username']
    clients:
      - client_id: huginn
        claims_policy: huginn   # add to the existing huginn client block
```

### Still open
- **Admin group**: what group means huginn-admin? (`OIDC_ADMIN_VALUE`) — depends
  on lldap/Authelia groups.
- **Non-root uploads**: container runs uid 10001; init the HDD volume dir
  ownership to 10001, or mount uploads at in-image `/uploads`.
