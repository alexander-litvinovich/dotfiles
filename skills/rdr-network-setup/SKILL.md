---
name: rdr-network-setup
description: Set up or inspect rundevrun local and optional tailnet networking when the user explicitly invokes this skill.
disable-model-invocation: true
---

# rundevrun network setup

Use the installed `rundevrun` binary and its `network` JSON commands. Setup is an explicit user action. Inspect current settings first, prepare a plan, show the exact plan, and apply only after the user directs setup.

## Workflow

1. Find the installed `rundevrun` executable and run `rundevrun network inspect --json` and `rundevrun network setup-check --json`.
2. For a settings change, prepare a version 1 desired JSON file, run `rundevrun network plan --file desired.json --json`, and show the returned plan before applying it with `rundevrun network apply --file plan.json --json`.
3. For tailnet setup, read locally known settings with `setup-check`. Then use `rundevrun network setup-plan --json` to check Cloudflare, DNS, certificate, and Serve state and present the proposed DNS, certificate, and Serve actions, including any terms URL. If the plan requires certificate terms, show that URL and get the user's explicit acceptance. Set `terms_accepted` to true only after that acceptance. Wait for the user to direct setup before running `rundevrun network setup-apply --file setup-plan.json --json`.
4. If a plan reports that it needs a Cloudflare token, ask the host secret-delivery tool for the token and pipe it directly to `rundevrun network credential-store cloudflare-dns-token`. To rotate a saved token, use `credential-replace` with the same stdin delivery. Never put the token in shell arguments, a JSON file, or command output. Record only `cloudflare-dns-token` as `credential_ref`, then create a fresh setup plan.
5. Verify with `rundevrun network setup-check --json`. Report the DNS, certificate, and Serve status, plus any second-device access check that remains.

If any baseline changes, inspect and create a fresh plan. Do not replay stale plans.

If setup reports an interrupted operation, run `rundevrun network setup-recover --json` first. When recovery reports an ambiguous resource, inspect it using provider status and ask the user before resolving it with `adopt-dns ID`, `adopt-serve`, `resolve-challenge ID`, or `resolve-finalization`. Pass only the resource ID returned by the provider. Resolving an ambiguous ACME finalization may consume another certificate issuance against CA rate limits. Do not adopt or delete a resource based on its name alone.

For an existing wildcard DNS record, do not adopt it automatically. Inspect the record ID and verify it is the intended DNS-only A record for the current Tailscale IP. After the user confirms, run `rundevrun network adopt-existing-dns ID`. If a proxy listener reservation remains after an interrupted reload, inspect `rundevrun proxy status`, then run `rundevrun proxy abort` to release the pending reservation before retrying.

If proxy reload reports `TLS mode or listener addresses changed; restart proxy`, run `rundevrun proxy stop`, then `rundevrun up` or initialize `rundevtop` again. This restarts the proxy and keeps application processes running.

To change the configured tailnet domain or Cloudflare zone, first run `rundevrun network cleanup-plan --json` and show its exact DNS and Serve removals. Get the user's direction, then apply that reviewed plan with `rundevrun network cleanup-apply --file cleanup-plan.json --json`. Cleanup removes only verified owned DNS and Serve forwarding. It preserves credentials and certificates. Then plan and apply the new domain settings, save its token reference if needed, and run a fresh setup plan. Do not edit ownership records manually.

The CLI accepts settings and plan JSON only. Never put a Cloudflare token, private key, authorization header, or secret value in a file passed to `--file`, command arguments, generated skill content, or a chat message. `credential_ref` is only a private credential file name under rundevrun's credentials directory. `credential-store` reads the token from stdin and prints only the reference name. Never inspect or scrape Cloudflare CLI credential files. Cloudflare browser login and the scoped API token used for DNS are separate credentials.

Local HTTP is always enabled. Tailnet sharing is per project and remains private to the tailnet. Never configure Funnel or a public tunnel. Do not modify DNS, issue certificates, change Tailscale Serve, install a local certificate authority, or change system resolver settings outside the shared rundevrun backend and an explicitly reviewed plan.
