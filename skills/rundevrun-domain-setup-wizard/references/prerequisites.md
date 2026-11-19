# Prerequisites and authentication

Fetch current primary instructions when installing or guiding login. Explain the
needed work, offer to perform it and obtain installation/update consent first.
Use the host's secret-delivery tool for credentials only the user can supply;
account login/2FA may need their participation. Keep secrets out of chat and logs.

## Rundevrun

Locate `rundevrun`, check `rundevrun version --json` and required `network` commands. Offer
installation/update when unavailable or incompatible. Prefer the current trusted
release/install procedure, or build from the user's trusted rundevrun checkout
following its `docs/INSTALL.md`, `go.mod` and `--help`. If the skill was installed
alone, locate that source with the user rather than assuming its parent directory
is the repository. Do not guess a release URL or silently upgrade Go/system tools.

Use a user-owned install directory where practical; retain previous binaries and
existing configuration. Build `rundevrun` and `rundevtop` where supplied by that version.
Recommend an update, but do not replace the user's working binary on discovery
alone. Runtime state and desired networking configuration remain separate.

## Tailscale

Start from the [official quickstart](https://tailscale.com/docs/how-to/quickstart)
and follow the linked macOS or Linux/distribution installation instructions.
Check the binary, daemon, login and `tailscale status --json`. If absent, offer
installation before making changes. Help the user authenticate to their intended
tailnet and verify this machine's assigned IP and `Running` state.

On macOS, locate an existing Tailscale application's CLI before offering another
installation variant. On Linux, use the instructions for the actual distribution
and verify its daemon/service as well as the CLI.

Inspect `tailscale serve status --json`. Existing port 443 use, insufficient Serve
permissions or a stopped daemon needs an explained recovery step. Preserve other
Serve entries. Changes to tailnet grants/ACLs or operator privileges require
separate user direction; do not reset Serve, enable Funnel or change system DNS.

## Cloudflare CLI and scoped token

Use the [official cf installation/login guide](https://developers.cloudflare.com/cf/get-started/).
Verify the installed `cf` is Cloudflare's CLI, not another program with the same
name; the `cloudflare` alias may be available. Check the actual selected package's
Node engine requirement. If the existing runtime is incompatible, offer a
supported isolated runtime rather than silently replacing system Node. Do not
pin a beta version or Node version from the author's machine into the workflow.

Use `cf cli search` to discover commands, keeping search queries generic and free
of domains, emails and IDs. Use the discovered command's help/schema for flags.
Authenticate with `cf auth login`, using its supported encrypted/keyring storage
where available. Request only the OAuth permissions needed for discovery and
token creation where supported; let the user review the actual authorization
screen. Confirm login with `cf auth whoami`. Never read/export the CLI's stored
OAuth credential files.

Look up the exact user's zone through `cf zones list --name DOMAIN`; verify one
active, unpaused zone and its public delegation. A configured credential reference
may already be usable: inspect and plan through rundevrun before creating another token.
If account or zone selection is ambiguous, ask the user and use that choice for
the CLI session; do not select the first account or guess a zone ID.

For a new credential, explain the token's zone and permissions before creation.
Use Zone Read plus DNS Write/Edit limited to that zone, so rundevrun can issue and renew
certificates without depending on the CLI login. The following bundled helper
captures the token internally and passes it directly to rundevrun's stdin:

```sh
node scripts/create-cloudflare-token.mjs --zone example.com --credential-ref cloudflare-dns-token
node scripts/create-cloudflare-token.mjs --zone example.com --credential-ref cloudflare-dns-token --apply
```

Resolve `scripts/` relative to this skill, not the user's current directory. Run
the helper with a Node version compatible with cf; it prepends that Node's
directory to PATH for child processes. `--cf` and `--rundevrun` select executable paths.
The first invocation is a read-only policy preview. Run `--apply` only within the
user's authorized token-creation step. It creates a token and stores it under
the chosen credential reference, not DNS/Serve resources. It refuses existing
references and attempts to revoke its newly created token if storage fails,
reporting its ID for recovery if revocation cannot be confirmed. Keep its
non-secret output for the desired-settings step. Inspect failures before retrying
an ambiguous creation; do not create tokens repeatedly.

If OAuth cannot create tokens, guide the user through the current official
[token creation instructions](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/).
Use host secret delivery to feed the scoped token to
`rundevrun network credential-store REF` over stdin, or a local masked input mechanism
if the host has no secret tool.
Never ask them to paste a token into chat. Never print `cf user tokens create`
output directly, put secret values in arguments/plans or use a Global API Key.

Existing references must be reused or explicitly rotated with `credential-replace`;
the helper does not overwrite them. Continue only with an active zone and a
usable private credential reference. Record only the reference and zone ID in
desired configuration; collect the CA email separately.
