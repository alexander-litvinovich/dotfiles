---
name: tlg
description: >
  Sends one brief Telegram push notification via telegram-notify when a task
  finishes. Use when the user invokes /tlg or $tlg, asks to notify them in
  Telegram, send them a message, ping in tlg, or says "скинуть в телегу", or
  when a long multi-step process completes and they wanted a ping at the end.
argument-hint: "[message]"
---

# tlg

Ping the user on Telegram once. Not a log stream.

## When to send

- **Once per invocation**, at the very end of the work (after the normal chat reply is ready).
- Send when triggered explicitly (`/tlg`, `$tlg`, or natural-language requests above), or when the user asked for a Telegram ping and a long or multi-step task just finished.
- **Do not** send progress updates, intermediate steps, or more than one message per task.

## How to send

```bash
telegram-notify "your message here"
```

Requires `TG_BOT_TOKEN` and a learned chat or `TG_CHAT_ID`. Setup: [cmd/telegram-notify/README.md](../../cmd/telegram-notify/README.md).

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

If `telegram-notify` exits non-zero, tell the user once in chat what failed (stderr is enough). Point them to [cmd/telegram-notify/README.md](../../cmd/telegram-notify/README.md). Do not retry in a loop.

## Examples

**After fixing a bug (user said "ping me in tlg when done"):**

```bash
telegram-notify "Done: fixed login redirect loop"
```

**Immediate ping with custom text:**

User: `$tlg tests green on main`

```bash
telegram-notify "tests green on main"
```
