---
name: rundevrun
description: Runs, switches, stops and inspects a project's dev services across Git branches and worktrees with `rundevrun`. Use when the user wants to start or stop an app, run the UI or API from a branch, PR or worktree, move running services to another worktree, check what is running or on which port, read service logs, or mentions rundevrun, rundevtop, rdr, `up`, `down` or `switch`. Do NOT use to onboard a repo that has no rundevrun config (use rdr-onboard) or to set up networking (use rdr-network-setup).
---

# rundevrun

Run dev services from any branch or worktree through `rundevrun`. Do not start
them with a package manager, `kill` or `lsof` instead. rundevrun records what it
started, so only it can stop or move the processes safely.

Add `--json` to every command you parse. Prefix shell commands with `rtk` if the
user's global instructions require it.

## Find the project and worktree

1. From inside the repo, run `rundevrun config check --json`. It reports
   `config_mode`, `worktree_root` and every service with its `state`.
   Take service names from this output. Never assume them.
2. `config_mode` of `external` means the config lives under
   `~/.config/rundevrun/`. Never add a `rundevrun.toml` to the repo for it.
3. If the check says there is no config, stop and use `rdr-onboard`.
4. `rundevrun worktrees --json` lists worktrees. Pick the one the user named by
   branch, directory name or PR number. If nothing matches, report it. Do not
   create a worktree unless the user asks.
5. `rundevrun status --json` shows what runs now, across all projects.

## Commands

```text
rundevrun up SERVICE... [--worktree W]      start services and their dependencies
rundevrun down [SERVICE...] [--worktree W]  stop services; no names means this worktree
rundevrun restart SERVICE...
rundevrun switch SERVICE... --project P --worktree SRC --to-worktree DST
rundevrun switch --all-services --project P --worktree SRC --to-worktree DST
rundevrun logs SERVICE --tail 100
rundevrun open SERVICE                      open the registered route
rundevrun down --all                        stop everything in every project
```

Run `up` or `switch` with `--dry-run --json` first when a port clash or a
dependency is likely. `--worktree` and `--project` accept an ID, a path or a name.
Use IDs from `rundevrun projects --json` when a name is ambiguous.

## Common tasks

**Run an app from a branch.** Find the worktree, then `rundevrun up SERVICE
--worktree W --json`. Verify with `status`.

**Move what is running to another branch.** Use `switch --all-services` for
everything in the source worktree, or name the services. It restarts them and
their consumers in the target. Do not run `down` first.

**Something fails to start.** Read `rundevrun logs SERVICE --tail 200`, then
`rundevrun doctor --json` for routes and proxy problems. A log line such as
"already running" or "address in use" means another process owns the port.
Follow the port rule below.

## Rules

- A service with a fixed port can run from one worktree at a time. `up` for the
  same service in a second worktree clashes. Use `switch` to move it.
- If a port belongs to a process rundevrun did not start, report the conflict
  and the owning process. Leave it alone. Stop it only after the user agrees.
  Then run `up` again.
- Prefer the `direct_url` from a dry-run or `status` when the app does auth with
  a fixed callback URL. The `*.localhost` route goes through the proxy and a
  fixed callback or base-URL setting will not match it.
- Do not edit rundevrun's state files. They sit beside the run logs, which
  `up --json` lists under `log_paths`. Reading them is fine. Recover
  with `up`, `down` and `status`. After an interrupted `switch`, inspect the
  journal in `status`, then `up` or `down` explicitly.
- Do not install, share publicly (`share`) or change networking unless asked.
- Finish by running `rundevrun status --json` and tell the user which service
  runs from which worktree and on which URL.
