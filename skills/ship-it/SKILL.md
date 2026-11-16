---
name: ship-it
description: Explicitly ship current changes through a branch, commit, push, PR, and an optional ponytail-review subagent. Use asis, as is, or a to skip the subagent review. Use prep or p to leave the PR open; otherwise merge when ready.
---

# Ship it

Run only on an explicit invocation of `ship it`, `$ship-it`, or `/ship-it`.
An explanation or mention of this workflow is not an invocation.

| Invocation | Completion |
| --- | --- |
| `ship it` | Reviewed, fixed, verified and merged PR. |
| `ship it prep` or `ship it p` | Reviewed, fixed and verified PR, left open with its link. |
| `ship it asis`, `ship it as is`, or `ship it a` | Verified and merged PR, without a review subagent. |
| `ship it prep asis` or `ship it p a` | Verified PR, left open without a review subagent. |

Accept these options with `$ship-it` or `/ship-it` too, including quoted
arguments and either option order. Treat `asis`, `as is`, and standalone `a`
as equivalent review-bypass options. They skip the Review and fix section;
branch preparation, validation, commits, pushes, and PR creation still apply.
Independently, `prep` or `p` leaves the PR open; without either, merge when ready.

The invocation authorizes the commits, pushes, PR creation, review
fixes and, unless preparing only, merge. Proceed without another confirmation
when the shipping scope is clear.

## Prepare the branch

Read the repository's agent instructions and inspect status, the current branch,
and the diff, including untracked files. Determine the PR base from repository
instructions or the remote's default branch.

Discover the remote hosting service and use its available connector or CLI.
Here, PR also means a merge request on hosts that use that term. Follow each
repository's authentication, validation, and merge rules; derive commands and
account selection from that repository rather than assuming a particular stack,
branch name, hosting service, or user account. If the host has no PR workflow or
the required access is unavailable, report the blocker before publishing.

Keep an existing dedicated task branch. If on the default branch, another
protected integration branch, or a detached HEAD, create a descriptive task
branch at the current HEAD while preserving all changes. When the current
branch's purpose is unclear, inspect its commits and existing PR before deciding.

Review the intended shipping changes, run `git diff --check` and the checks
appropriate to them, then stage explicit paths and commit with a descriptive
message. Preserve unrelated work. If the intended changes are already committed,
continue with those commits; do not create an empty commit. If there is no diff
to ship against the base, report that and stop.

Push the dedicated branch to its corresponding remote branch, establishing
upstream tracking when needed. Follow repository authentication instructions
for every hosting-service command.

## Open the PR

Create a PR against the selected base, or reuse the open PR already associated
with this branch. Write its title and description around the final behavior,
the problem solved, and relevant validation. Follow the repository's PR template
when present. With `gh`, put multiline descriptions in a temporary file and use
`--body-file`.

Follow the host's PR-linking protocol immediately after PR creation and each
push to an existing PR. Keep the full PR URL for the final response.

## Review and fix

If a review-bypass option was passed, skip this section and continue to Finish.
Do not launch a review subagent or replace it with another review pass.

Run one subagent to apply the installed `ponytail-review` skill to the full PR
diff against its base. Give it the repository path, base and head, the originating
request or spec, and the PR URL. Ask for findings only, with file locations and
specific simplifications. The reviewer shares the checkout with other work and
must leave files unchanged.

Use the host's supported subagent mechanism and wait for its findings. If
`ponytail-review` or subagent execution is unavailable, report the blocker and
leave the PR open rather than claiming this step passed.

Apply the suggested simplifications that preserve the requested behavior.
Investigate any recommendation that conflicts with the spec or would break
correctness, and explain why it was not applied. Preserve concurrent edits.
Run checks appropriate to the fixes and `git diff --check`, then commit and push
the fixes to the same branch and PR. If there were no applicable findings, skip
the empty fix commit. Update the PR description when the final change or
validation differs from what it currently describes.

## Finish

For `prep` or `p`, leave the PR open and return its link, validation results,
and any outstanding CI or review requirements. Do not merge it or schedule an
automatic merge.

Without `prep` or `p`, merge the PR using the repository's accepted merge method
once all intended changes are pushed and the relevant checks and required reviews pass.
When waiting for CI or reviews, use the host's PR monitoring mechanism if
available and continue when notified. If blocked without a supported way to
continue, return the PR link and the specific blocker. Do not report completion
until the hosting service confirms the merge.

Keep the final reply brief: PR link, merged or prepared status, relevant checks,
and any unresolved review findings.
