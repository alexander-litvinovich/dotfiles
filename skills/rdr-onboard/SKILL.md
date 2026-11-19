---
name: rdr-onboard
description: Onboard or re-onboard a repository to rundevrun, discover and update existing shared or external configuration, and configure fixed-port services that must run in only one worktree.
---

# Onboard a repo to rundevrun

Configure the installed `rundevrun` for the repository's existing development commands.
Use `rundevrun --help` to check available commands. If `config external` is unavailable,
report that External mode needs a newer rundevrun before changing files.

## Discover existing setup

For onboarding and re-onboarding, run `rundevrun config external --json` from
the repository before choosing a mode. Inspect the returned `config_file` if it
exists. This locates the project's config in rundevrun's settings folder,
including an `XDG_CONFIG_HOME` override. Also inspect `rundevrun.toml` in the
selected checkout and existing run instructions. Keep secret values out of output.

An existing external config takes precedence over the checkout's config.
Run `rundevrun config check --json` to inspect the active source; if invalid,
repair that source rather than falling back or replacing it. Re-onboarding
updates the active config in place, preserving unrelated services, environment
file selections, hooks, and user settings. Reuse an established mode unless the
user requests a change. Completion means the active config and its precedence
are known before writing anything.

## Choose the mode

Honor the requested or established mode. If neither exists, ask whether the team
adopts rundevrun or the setup must stay outside the repo.

- **Shared:** create or update `rundevrun.toml` at the project root for the team to commit.
- **External:** keep runner configuration under rundevrun's user config directory.
  Leave the checkout, Git metadata, package scripts, ignore files, hooks, and
  agent instructions unchanged. Service commands may still write their normal
  build output when the user runs them.

These are the only two modes. A locally ignored `rundevrun.toml` is still a repo file.

## Inspect and configure

Read existing run instructions, manifests, environment examples, and service
entry points. Reuse the project's commands and dependency tooling. Identify
which services are needed, their working directories, dependency order, and
how they accept a loopback host and a port. Check existing runner configuration
before replacing anything.

Use the same version 1 TOML format in either mode:

```toml
version = 1
[project]
name = "example"
[services.worker]
command = "existing-development-command"
cwd = "."
```

Service names use lowercase letters, digits, underscores, and hyphens, beginning
with a letter. `cwd` is relative to the selected worktree and must exist there.
Omit `cwd` for the root. Services without ports default to `process`.
For an HTTP service whose address can change, use `port = "auto"` and pass
`{host}` and `{port}` through the app's supported flags or environment.
For a fixed address, follow the section below. Add an existing health endpoint
if available. Commands place rundevrun placeholders outside shell
quotes. Use `depends_on = ["service-name"]` for actual startup dependencies.
Keep credentials in the project's existing environment setup.

### Fixed addresses and singleton services

Inspect application URL settings, auth callback URLs, cross-app links, and run
instructions for address constraints. Examples include `NEXTAUTH_URL`,
`AIRM_APP_URL`, and `AIWB_APP_URL`. Preserve their exact scheme, hostname, and port;
`localhost` and `127.0.0.1` may have different registered callbacks. Describe auth
backend redirect support as unverified unless inspected.

When the workflow requires one active worktree per service, preserve the fixed
port and configure `singleton = true` on that service. Check support with the
installed version before applying it; if unsupported, explain the required
upgrade and preserve the working configuration. Starting the service in another
linked worktree will automatically stop the previous instance before starting
the target. Explain this behavior and the brief interruption in the setup report.
Separate clones remain separate projects. A port owned by an unmanaged process
remains a conflict.

For example, retain an existing `NEXTAUTH_URL=http://localhost:8010` and configure
`port = 8010` with `singleton = true`. Use the direct localhost URL for login and
cross-app links when that is what the auth backend accepts. Introduce routed
subdomains only when the app and auth configuration support them. A fixed port
alone does not authorize stopping another instance; enable singleton behavior
when it matches the requested workflow.

### Shared environment files

Inspect the main Git checkout and the selected worktree for actual dotenv files,
including ignored files such as `.env`, `.env.local`, and service-specific files.
Exclude templates such as `.env.example` and `.env.sample`. Use filenames when
discussing candidates; keep secret values out of chat and runner configuration.

Use `project.env_files = [".env"]`, with paths relative to each checkout root.
Check the installed version's config documentation or schema for support before
writing this setting. If unsupported, report the detected files and proposed
selection, explain that a newer rundevrun is needed, and preserve the existing
env setup.

On re-onboarding, preserve the existing `env_files` selection unless changing it
is requested or necessary for the setup. For a new selection, when supported,
opt in the sole candidate automatically. If several distinct
paths exist, ask which to share and in what order before configuring them. Count
matching main-checkout and worktree paths as one candidate. If none exist, leave
the setting unset and report any environment setup the app still requires.

For selected files, load all shared files from the main checkout first, then all
files from the worktree. Within each checkout, later files win; worktree values
override all shared values. Missing files are allowed and
explicit service `env` wins over file values. Explain that worktree files are
optional overrides. Keep existing files intact; this setup needs no copies or
symlinks. Apply the same selection process in Shared and External modes.

For Shared mode, update or create `rundevrun.toml` in the repo. If switching from
an existing external config at the user's request, explain its precedence and
preserve its contents when moving it aside to activate Shared mode.

For an existing External setup, edit the discovered `config_file` directly.
For a new External setup, write the proposed TOML to a temporary file outside
the repo, then run these commands from the repo:

```sh
rundevrun config external --file /absolute/path/to/proposed.toml --dry-run --json
rundevrun config external --file /absolute/path/to/proposed.toml --json
```

The result gives `config_file` and `worktree_root`. Import refuses to overwrite
an existing external config. External config applies to all linked
worktrees and takes precedence over their `rundevrun.toml` files. Separate clones have
separate configs. Remove the temporary proposal after importing it.

## Verify and register

Run `rundevrun config check --json` and inspect `config_mode`, `config_file`, and
`worktree_root`. Run `rundevrun up SERVICE... --dry-run --json` for the intended services.
Fix validation or planning errors before registering with `rundevrun register . --json`.
Registration runs no project commands and lets rundevtop recognize the repo.
For singleton services, inspect any planned source stops and affected consumers;
dry-runs must leave running services unchanged. Re-onboarding uses these same
checks even when the project is already registered.

Start services only when starting them is in the user's requested scope.
When authorized, use `rundevrun up SERVICE... --json`, inspect status and logs, and
resolve startup errors. Otherwise finish with the validated config and commands
to start services and open `rundevtop`.

Report the mode, config path, configured services, validation result, and any
environment setup still needed. Include fixed URLs and singleton handoff behavior
when configured. In External mode, verify onboarding left repo
files and Git metadata unchanged, accounting for changes present before the task.
