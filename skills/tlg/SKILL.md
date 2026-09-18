---
name: tlg
description: >
  Sends one brief Telegram push notification via telegram-notify when a task
  finishes, or blocks on a Telegram question by adding --prompt to a
  telegram-notify command when
  the agent needs the user's input. Use when the user invokes /tlg or $tlg,
  asks to notify them in Telegram, send them a message, ping in tlg, or says
  "скинуть в телегу"; when a long multi-step process completes and they
  wanted a ping at the end; or when the agent needs to ask the user something
  and wait for their answer over Telegram instead of blocking chat; or when the
  user asks to send a plan or proposal to Telegram for approval, in which case
  use the interactive --prompt flow with approval buttons.
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
telegram-notify --text "your message here"
```

Run `telegram-notify` without arguments for first-time setup. Full setup and
override instructions: [cmd/telegram-notify/README.md](../../cmd/telegram-notify/README.md).

## First-time setup

No preflight check: just attempt the send. When the util is not configured,
any send or `--learn` exits 1 and prints one line to stdout:

```
not configured: missing bot token
not configured: missing chat ID
not configured: missing bot token and chat ID
```

Nudge the user based on what is missing:

- **chat ID only** — run `telegram-notify --learn` and ask the user to send `/start` to the bot from their Telegram app. The command blocks until the message arrives (Ctrl+C stops it), then saves the chat ID and sends a test message.
- **bot token only** — ask the user for the bot's API token (they create a bot via @BotFather), run `telegram-notify --set-token TOKEN`, then `--learn` if the chat ID is also missing.
- **both** — offer two paths: the user runs `telegram-notify` in a terminal and goes through the interactive chat wizard (preferred), or hands you the token for `--set-token` + `--learn` as above.

Ask for the token only in a private chat with the user; it grants full
control of the bot. Tokens passed via `--token` or `TG_BOT_TOKEN` apply to
one invocation only and are never saved.

## Asking the user a question

When you need an answer from the user (not a one-way ping), block on:

```bash
telegram-notify --text "your question here" --prompt
```

This sends the question, waits up to 5 minutes for a plain-text reply or an inline button tap, and prints the answer on stdout (button label or reply text). A button tap edits the question message to append `Answer: <label>` and reacts with 👍, so the choice stays visible; a text reply gets a 👀 reaction and the buttons are removed instead. If the user reacts to a "still there?" check-in near the deadline, the wait extends by 5 more minutes. On timeout the command exits non-zero, the chat gets a timeout notice, buttons are removed, and stdout is empty. Treat that like any other missing answer: tell the user in chat and don't retry in a loop. Full details: [cmd/telegram-notify/README.md](../../cmd/telegram-notify/README.md).

For yes/no or fixed choices, add `--button` (repeatable):

```bash
answer=$(telegram-notify --text "Proceed with deploy?" --prompt --button Yes --button No)
```

Use `--text "question" --prompt` without `--button` for a free-form answer.

Use this only when you truly need their input; keep using a plain `telegram-notify` message for completion pings.

### Sending a plan or proposal for approval

When the user asks you to send a plan, proposal, or set of changes to Telegram for them to check or approve, you **must** use `--prompt` with explicit approval buttons — not a one-way `telegram-notify` message followed by silent waiting. This lets the user approve or reject directly from Telegram. Capture the answer and act on it: proceed on approval; incorporate the feedback or halt on rejection.

```bash
answer=$(telegram-notify --text "Plan ready: migrate auth to OAuth in 3 steps. Approve?" --prompt --button Approve --button "Request changes")
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

If `telegram-notify` exits non-zero (including a `--prompt` timeout), tell the user once in chat what failed (stderr is enough). Point them to [cmd/telegram-notify/README.md](../../cmd/telegram-notify/README.md). Do not retry in a loop.

## Examples

**After fixing a bug (user said "ping me in tlg when done"):**

```bash
telegram-notify --text "Done: fixed login redirect loop"
```

**Immediate ping with custom text:**

User: `$tlg tests green on main`

```bash
telegram-notify --text "tests green on main"
```

**Need a decision before continuing (user is away from chat):**

```bash
answer=$(telegram-notify --text "Deploy to prod now, or wait for review?" --prompt --button Deploy --button Wait)
```
