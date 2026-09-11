# telegram-notify

Personal Telegram notifier for scripts and coding agents. It reads the bot
token from `TG_BOT_TOKEN` and sends plain-text messages to a learned private
chat or to the chat specified by `TG_CHAT_ID`.

## Setup

Create a bot via [@BotFather](https://t.me/BotFather) (`/newbot`) and copy
the token it gives you into `TG_BOT_TOKEN`. No other setup on the Telegram
side is needed.

To connect your Telegram account, start learning mode and then send `/start`
to the bot in a private chat:

```bash
export TG_BOT_TOKEN='your-bot-token'
telegram-notify --learn
```

The command stores the chat ID in
`$XDG_CONFIG_HOME/telegram-notify/config.json`, or
`~/.config/telegram-notify/config.json` when `XDG_CONFIG_HOME` is unset. The
bot token is never written to disk.

Send a notification by passing the message as one or more arguments:

```bash
telegram-notify "The agent finished the task"
telegram-notify Tests passed on "$HOST"
```

Set `TG_CHAT_ID` to override the learned chat for one invocation:

```bash
TG_CHAT_ID=-1001234567890 telegram-notify "Deployment finished"
```

Learning mode uses Telegram long polling and cannot run while the bot has an
active webhook. Press `Ctrl+C` to stop waiting for `/start`.

## Prompting for a reply

Ask a question and block until the user answers in Telegram (for agents and
scripts that need input):

```bash
telegram-notify --prompt "Which environment should I deploy to?"
```

The command sends the question, then waits up to five minutes for a plain-text
reply in the configured chat. When a reply arrives, the bot reacts with 👀 on
that message and prints the reply text on stdout.

About 30 seconds before the deadline, if there is still no reply, the bot sends
a check-in message. React to that message with any emoji to get five more
minutes; this can repeat as long as you keep reacting before time runs out.

If the wait ends with no reply, the bot sends `Request timed out waiting for a
reply.` to the chat, prints an error on stderr, and exits with a non-zero
status. Nothing is written to stdout.

`--prompt` requires a private or group chat ID (learned config or numeric
`TG_CHAT_ID`). Channel usernames (`@channel`) are not supported. Only text
messages count as answers; photos, stickers, and other non-text messages are
ignored.

Prompt mode also uses long polling and cannot run while the bot has an active
webhook.
