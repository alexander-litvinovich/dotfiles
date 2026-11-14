---
name: orchestrate
description: Plan or build with harness-specific Orchestrator and Builder models, with optional Supervisor review. Explicit invocation only.
disable-model-invocation: true
argument-hint: "plan [sup] | build [plan path or text]"
---

# Orchestrate

Activate only when the user explicitly invokes `/orchestrate` or `$orchestrate`.
Mentioning orchestration or working on this skill does not activate it.

## Arguments

- `plan`: turn the preceding conversation into a detailed implementation plan.
- `build`: implement a supplied plan using its Supervisor decision.
- `sup`: valid with `plan` only. Require Supervisor review of both the plan
  and the completed implementation, and record that requirement in the plan.
- A plan may be a file path, attached document, inline text, or an unambiguous
  plan already supplied in the conversation. An explicit output path overrides
  the default plan location.

Require exactly one mode. Recognize `sup` as a separate argument, not as text
inside a plan or path. If the mode is missing or conflicting, ask which mode
to run before dispatching agents. With `build`, ask the user for a plan when
none is supplied; wait for it rather than inventing one. If `build sup` is
used, explain that build reads this decision from the plan and ask the user
to confirm the intended plan decision before dispatching agents.

## Harness and models

Identify the executing harness from authoritative session metadata and its
agent tools. A model name, installed executable, or repository configuration
alone does not identify the harness. A Codex session hosted by T3 Code is
Codex. For an unknown or ambiguous harness, ask before choosing models.

| Harness | Supervisor | Orchestrator | Builder |
| --- | --- | --- | --- |
| Claude Code | Fable, Opus | Opus | Sonnet, Haiku |
| Codex | Astra | Sol | Terra, Luna |

Resolve these model families against the current runtime's available models
and aliases. Use concrete identifiers returned by the runtime; never invent
an identifier or silently substitute an unlisted family. For Supervisor,
prefer the first available listed family. Use Terra or Sonnet for substantial
implementation, and Luna or Haiku for small, well-bounded changes. Availability
may select the other listed Builder. If a required role has no available
listed model, report the blocked role and ask the user how to proceed.

Use the host's supported delegation tools and obey its routing, waiting, and
agent cleanup rules. Native delegation fits when it supports the requested
model and settings; use T3 delegation when the host requires it. Discover
capabilities before the first T3 delegation. If delegation or model selection
is unavailable, report the limitation before starting the affected work.

The current agent may act as Orchestrator only if its model matches that role.
Otherwise delegate to the selected Orchestrator and pass the mode, review decision,
user requirements, relevant conversation, repository path, and supplied plan.
Explicitly authorize that Orchestrator to delegate Builder tasks and any
required Supervisor review. Carry over user constraints and applicable repository
instructions. If the host cannot support this delegation structure, explain
the limitation and ask for a supported arrangement rather than impersonating
the missing roles. Report the detected harness and selected role models.

## Plan

The Orchestrator owns the plan. Read the preceding dialogue and inspect the
relevant code and repository instructions. Capture the user's intended outcome,
accepted decisions, and constraints. Ask only about missing information that
would materially change the implementation; label remaining assumptions.

Write a plan that a later Orchestrator can execute without the original chat:

- Goal, scope, constraints, and acceptance criteria.
- Current behavior and relevant files or modules, grounded in inspection.
- Proposed changes, including interfaces or data changes where needed.
- Ordered tasks with dependencies, file or module ownership, selected Builder
  families, and a concrete completion condition for each task.
- Which tasks can run in parallel and which must wait for earlier work.
- Integration steps and validation commands tied to acceptance criteria.
- Risks, assumptions, and unresolved decisions that affect execution.

Include a `Supervisor decision` section with `supervisor_required: true` when
`plan` includes `sup`, or `supervisor_required: false` otherwise. Add a
`reason` explaining the user's choice and what final review will cover or
why the Orchestrator's checks are sufficient. This decision governs the later
build even in a new session. Record the role, not a fixed model identifier;
build resolves the Supervisor model for its executing harness.

Scale detail to the task. Builders may perform bounded read-only feasibility
checks when useful; planning does not authorize implementation changes.

Unless the user specifies another destination, create a unique directory under
the OS temporary directory and write `plan.md` there. Resolve its absolute path
and verify it is outside the repository, even if `TMPDIR` points inside it.
Choose another writable OS temporary location if necessary. Keep the plan
available after the invocation. When planning is complete, explicitly print
the plan's absolute path in the final user-facing response, including when
planning was delegated. Use the full path as the visible link text so the
user can both open and copy it.

With `sup`, send the complete draft, user requirements, and inspection evidence
to the Supervisor. Apply the review process below before delivering the plan.
Stop after delivering it; implementation requires a `build` invocation.

## Build

The Orchestrator reads the supplied plan and checks it against the current
repository state and instructions. Resolve material gaps or stale assumptions
before affected tasks begin. Preserve the plan's intent; ask the user about
changes to scope or product decisions.

Read `supervisor_required` and its reason from the plan before dispatching
agents. If the decision is missing, contradictory, or unclear, ask the user
whether Supervisor review is required, then record the answer and reason in
the plan. Honor an explicit user amendment to that decision. Report the
effective review decision with the selected models.

Dispatch implementation tasks to Builders using the selected model families.
Give each Builder its task, relevant plan section, acceptance criteria, owned
files or modules, dependencies, and validation expectations. Tell Builders they
are not alone in the codebase, must preserve others' edits, and must adapt to
concurrent changes. Keep overlapping writes sequential. Run independent tasks
in parallel when supported, then collect results before dependent work starts.

The Orchestrator manages dependencies, integration, and progress. Builders own
implementation and fixes. Route failed checks and integration defects to the
responsible Builder, then inspect the result and run relevant validation.
Complete the plan's acceptance checks and required repository checks. Distinguish
passing checks from checks that could not run.

When the plan requires Supervisor review, send the plan, final changes,
Builder results, and validation evidence to the Supervisor for final review
and decision. Otherwise, the Orchestrator makes that decision. Report completed work, validation results,
and any remaining blockers; claim completion only when the acceptance criteria
and requested final review are satisfied.

## Supervisor review

Create a Supervisor when `plan` includes `sup` or `build` reads a required
Supervisor decision from the plan. Give it enough
original context and evidence to judge the result independently. Ask it to
check alignment with user intent, plan completeness in `plan` mode, and
correctness and validation in `build` mode. Require a decision of `approved`
or `changes required`, with concrete findings and their locations.

The Orchestrator revises the plan or dispatches Builder fixes for blocking
findings, then requests a fresh Supervisor review with updated evidence.
The Supervisor makes the final decision. If resolution needs user input or
an unavailable capability, report the blocker and the outstanding findings.
Supervisor approval is a technical decision, not additional user authorization
for publishing, merging, deploying, or other external actions.
