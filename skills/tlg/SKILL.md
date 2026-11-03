---
name: tlg
description: >
  Sends one brief Telegram push notification via tlgme when a task finishes,
  or blocks on a Telegram question by adding --prompt to a tlgme command when
  the agent needs the user's input. Use when the user invokes /tlg or $tlg,
  asks to notify them in Telegram, send them a message, ping in tlg, or says
  "скинуть в телегу"; when a long multi-step process completes and they
  wanted a ping at the end; or when the agent needs to ask the user something
  and wait for their answer over Telegram instead of blocking chat; or when
  the user asks to send a plan or proposal to Telegram for approval, in which
  case use the interactive --prompt flow with approval buttons.
argument-hint: "[message]"
---

# tlg

Ping the user on Telegram once, or ask them a question and wait for the reply. Not a log stream.

## When to send

- **Once per invocation**, at the very end of the work (after the normal chat reply is ready).
- Send when triggered explicitly (`/tlg`, `$tlg`, or natural-language requests above), or when the user asked for a Telegram ping and a long or multi-step task just finished.
- **Do not** send progress updates, intermediate steps, or more than one message per task.

## When to prompt instead

- Use the prompt flow (below) only when you genuinely need the user's answer to proceed, and they're likely away from chat (e.g. a long task and they asked to be pinged, or they're only reachable via Telegram right now).
- Prefer asking directly in chat when the user is actively present; the prompt flow is for cases where Telegram is the more reliable channel.

## How to send

```bash
tlgme --text "your message here"
```

Run `tlgme` without arguments for first-time setup. Full setup and override
instructions: [tlgme README](https://github.com/bettertomorrow-dev/tlgme#readme).

## Quiet delivery

Use `--silent` when the user asks for a quiet, silent, or no-sound Telegram
notification. It suppresses the Telegram notification sound for that command.
Leave it out for the normal audible completion ping.

```bash
tlgme --text "Done: nightly build passed" --silent
```

For a quiet question, add it to the prompt command. The question, any
check-in, and the timeout notice all stay silent.

```bash
answer=$(tlgme --text "Deploy tonight?" --prompt --silent --button Yes --button No)
```

## Attaching an image or a file

An attachment replaces the normal completion ping. Do not send a second
message for the same task.

Use `--image` when the user needs an inline preview: screenshots, charts,
diagrams, and rendered output.

```bash
tlgme --text "Done: rendered the dependency graph" --image /tmp/graph.png
```

Use `--file` when the bytes matter or the result is not an image: logs,
reports, PDFs, archives, `.excalidraw` and `.tldr` source files, and
text-heavy screenshots that Telegram would blur.

```bash
tlgme --text "Done: attached the audit report" --file /tmp/report.pdf
```

Pass the path the agent just wrote whenever possible. `--image` and `--file`
also accept an HTTP(S) URL, base64 data, a base64 `data:` URI, or `-` for
stdin. Use `--filename report.pdf` with stdin or base64 when the displayed
filename matters.

`--text` becomes the attachment caption. Keep it to one short line and under
1024 characters; detailed results stay in chat. `--image` falls back to a
file with a warning when the upload exceeds 10 MB or is not an image.

## First-time setup

No preflight check: just attempt the send. When the util is not configured,
any send or `--learn` exits 1 and prints one line to stdout:

```
not configured: missing bot token
not configured: missing chat ID
not configured: missing bot token and chat ID
```

Nudge the user based on what is missing:

- **chat ID only** — run `tlgme --learn` and ask the user to send `/start` to the bot from their Telegram app. The command blocks until the message arrives (Ctrl+C stops it), then saves the chat ID and sends a test message.
- **bot token only** — ask the user for the bot's API token (they create a bot via @BotFather), run `tlgme --set-token TOKEN`, then `--learn` if the chat ID is also missing.
- **both** — offer two paths: the user runs `tlgme` in a terminal and goes through the interactive chat wizard (preferred), or hands you the token for `--set-token` + `--learn` as above.

Ask for the token only in a private chat with the user; it grants full
control of the bot. Tokens passed via `--token` or `TG_BOT_TOKEN` apply to
one invocation only and are never saved.

## Asking the user a question

When you need an answer from the user (not a one-way ping), block on:

```bash
tlgme --text "your question here" --prompt
```

This sends the question, waits up to 5 minutes for a plain-text reply or an inline button tap, and prints the answer on stdout (button label or reply text). A button tap edits the question message to append `Answer: <label>` and reacts with 👍, so the choice stays visible; a text reply gets a 👀 reaction and the buttons are removed instead. If the user reacts to a "still there?" check-in near the deadline, the wait extends by 5 more minutes. On timeout the command exits non-zero, the chat gets a timeout notice, buttons are removed, and stdout is empty. Treat that like any other missing answer: tell the user in chat and don't retry in a loop. Full details: [tlgme README](https://github.com/bettertomorrow-dev/tlgme#readme).

For yes/no or fixed choices, add `--button` (repeatable):

```bash
answer=$(tlgme --text "Proceed with deploy?" --prompt --button Yes --button No)
```

For visual approval, attach the preview to the question. The answer still
arrives on stdout:

```bash
answer=$(tlgme --text "Does this look right?" --image /tmp/preview.png \
  --prompt --button "Looks good" --button "Fix it")
```

Use `--text "question" --prompt` without `--button` for a free-form answer.

Use this only when you truly need their input; keep using a plain `tlgme` message for completion pings.

### Sending a plan or proposal for approval

When the user asks you to send a plan, proposal, or set of changes to Telegram for them to check or approve, you **must** use `--prompt` with explicit approval buttons — not a one-way `tlgme` message followed by silent waiting. This lets the user approve or reject directly from Telegram. Capture the answer and act on it: proceed on approval; incorporate the feedback or halt on rejection.

```bash
answer=$(tlgme --text "Plan ready: migrate auth to OAuth in 3 steps. Approve?" --prompt --button Approve --button "Request changes")
```

Summarize the plan in one short line in the `--text`; the full plan stays in chat.

## Message content

One short plain-text line. Telegram gets plain text only (no markdown).

| Situation                             | Message                                                |
| ------------------------------------- | ------------------------------------------------------ |
| Success                               | `✅: <what was accomplished>`                          |
| Failure                               | `❌: <task> - <short reason>`                          |
| User gave text after `/tlg` or `$tlg` | Send that text **verbatim**                            |
| `/tlg` or `$tlg` with no extra text   | Brief status of the current task (same rules as above) |

Keep it push-notification sized: a few words to one short sentence. No stack traces, diffs, or multi-paragraph summaries.

## Errors

If `tlgme` exits non-zero (including a `--prompt` timeout), tell the user once in chat what failed (stderr is enough). The same applies to invalid attachments and caption-length errors. Point them to [tlgme README](https://github.com/bettertomorrow-dev/tlgme#readme). Do not retry in a loop.

## Examples

**After fixing a bug (user said "ping me in tlg when done"):**

```bash
tlgme --text "Done: fixed login redirect loop"
```

**Immediate ping with custom text:**

User: `$tlg tests green on main`

```bash
tlgme --text "tests green on main"
```

**Need a decision before continuing (user is away from chat):**

```bash
answer=$(tlgme --text "Deploy to prod now, or wait for review?" --prompt --button Deploy --button Wait)
```
