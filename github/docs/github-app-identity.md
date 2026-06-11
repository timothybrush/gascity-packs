# GitHub App Identity Resolver Contract

`github_app_identity` lets an addressed GitHub request reply as the profile
that handled the request. The public pack treats the identity as an opaque
deployment-local key. Secret stores, file paths, and organization policy belong
behind the resolver command, not in `rules.toml`.

## Identity String

An identity must match:

```text
[A-Za-z0-9][A-Za-z0-9._:-]{0,127}
```

Examples: `mayor`, `review-bot`, `prod:mayor`, `team_a.mayor`.

Do not put secret-store paths, file paths, GitHub private keys, tokens, or
installation tokens in `github_app_identity`.

## Resolver Command

Set `GITHUB_INTAKE_IDENTITY_RESOLVER` to an executable command. github-intake
calls it as:

```bash
$GITHUB_INTAKE_IDENTITY_RESOLVER <identity>
```

The command must:

- accept exactly one identity argument
- write exactly one JSON object to stdout
- write diagnostics to stderr
- exit nonzero when the identity cannot be resolved
- never log private key material, tokens, or webhook secrets to stderr

## Schema V1

The resolver JSON object must include:

```json
{
  "schema_version": "github-intake.github-app-identity.v1",
  "app_id": "123456",
  "private_key_pem": "-----BEGIN PRIVATE KEY-----\\n...\\n-----END PRIVATE KEY-----\\n"
}
```

Optional fields:

```json
{
  "installation_id": "987654",
  "webhook_secret": "...",
  "client_id": "Iv1.example",
  "client_secret": "...",
  "slug": "my-app",
  "html_url": "https://github.com/apps/my-app",
  "name": "My App",
  "owner": "my-org",
  "ready": true
}
```

If `ready` is present, it must be `1`, `true`, or `yes`. For
`gc github comment-issue`, `installation_id` may come from either the
resolver JSON or the command's `--installation-id` flag.

## Quick Start: Local File Resolver

For a small city or local test, store identity JSON outside committed source and
use the example resolver:

```bash
mkdir -p config/github-intake/identities
chmod 700 config/github-intake/identities
cat >config/github-intake/identities/mayor.json <<'JSON'
{
  "schema_version": "github-intake.github-app-identity.v1",
  "app_id": "123456",
  "installation_id": "987654",
  "private_key_pem": "-----BEGIN PRIVATE KEY-----\\nreplace me\\n-----END PRIVATE KEY-----\\n",
  "ready": true
}
JSON
chmod 600 config/github-intake/identities/mayor.json
export GITHUB_INTAKE_IDENTITY_RESOLVER="/abs/path/to/github-intake/examples/file_identity_resolver.py"
```

Then configure an address:

```toml
[[repo.address]]
address = "@mayor"
pool = "mayor"
profile = "mayor"
github_app_identity = "mayor"
formula = "github-addressed-message"
ack = true
```

For production, replace the file resolver with a resolver backed by your secret
store. Keep the public identity stable and move all store-specific naming and
credential rotation policy into the resolver.

## Publisher Command

The publisher is the write-side mirror of the resolver. When
`GITHUB_INTAKE_IDENTITY_PUBLISHER` is set (process environment or city
`[workspace.env]`), the service runs it after credentials are captured — the
manifest onboarding callback and the admin `/v0/github/app/import` endpoint —
so freshly created App credentials land in your secret store instead of being
stranded in the service's local `config.json`.

Invocation contract:

- The command is invoked with the identity string as its only argument, the
  same shape as the resolver.
- The identity JSON document (schema v1, see above) arrives on stdin with
  `schema_version` stamped; empty fields are omitted.
- Exit 0 means published. Any other exit reports a publish failure, which the
  service logs and surfaces but never treats as fatal — credential capture
  must not be broken by a misbehaving store. With no publisher configured the
  hook is a clean no-op.

## Quick Start: Local File Publisher

`examples/file_identity_publisher.py` is the write-side pair of the file
resolver: it persists the incoming identity to
`$GITHUB_INTAKE_IDENTITY_DIR/<identity>.json` (0600, atomic, merge-on-update
so operator-set fields like `permissions` survive a partial republish), which
is exactly the file the example resolver reads back:

```bash
export GITHUB_INTAKE_APP_IDENTITY="mayor"
export GITHUB_INTAKE_IDENTITY_PUBLISHER="/abs/path/to/github/examples/file_identity_publisher.py"
export GITHUB_INTAKE_IDENTITY_RESOLVER="/abs/path/to/github/examples/file_identity_resolver.py"
```

With both set, the manifest onboarding flow round-trips with no secret store
at all: create the App in the browser, the callback publishes the credentials
to the identity file, and the resolver (service startup, sync-app order)
reads them back. The `ready` flag in the file is derived from field
completeness, never trusted from the caller. For production, swap the
publisher for one backed by your secret store and keep the same contract.
