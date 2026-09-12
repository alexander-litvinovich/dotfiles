# telegram-notify

`telegram-notify` sends plain-text Telegram messages and can wait for replies.

## First run

Create a bot through [@BotFather](https://t.me/BotFather) with `/newbot`, then run:

```bash
telegram-notify
```

The setup wizard is a chat with the tool itself. It greets you with the current
version, asks you to paste the bot token (masked as bullets, never echoed),
and checks it against Telegram. It then shows the bot's username, asks you to
send `/start` to the bot in a private chat, and waits five minutes. When
`/start` arrives, it saves the settings, sends a test message to your chat,
and prints the commands you can use. If `/start` doesn't arrive in time, press
Enter to wait another five minutes or Ctrl+C to give up (exit code 130).

The finished transcript stays in your terminal history, and Ctrl+C works at
any step. If the Telegram API is unreachable, the wizard shows the CLI
commands for setting the token and chat ID directly instead.

The config lives at `$XDG_CONFIG_HOME/telegram-notify/config.json`, or
`~/.config/telegram-notify/config.json` when `XDG_CONFIG_HOME` is unset. The
file is readable only by its owner and also stores the bot's username.

If either setting is missing, running `telegram-notify` resumes setup and asks
only for the missing value. A token from `TG_BOT_TOKEN` or `--token` is used
for the session but never written to the config. Once setup is complete,
running the command without arguments shows help.

If `telegram-notify` is not on your `PATH`, the wizard shows an export line
for zsh, bash, or fish. It never edits shell files.

The wizard needs an interactive terminal to read a missing token. For scripts,
provide `TG_BOT_TOKEN`, `--token`, or save it first with `--set-token`.

Commands that need settings (`--text`, `--prompt`, `--learn`) print
`not configured: missing <what>` to stdout and exit 1 when a required value
is unavailable from flags, environment, or saved config. Scripts and agents
can react to that line instead of parsing stderr.

## Send a message

```bash
telegram-notify --text "The agent finished the task"
telegram-notify --text "Tests passed on $HOST"
```

Text must be passed with `--text`. Positional messages are not supported.

## Ask for a reply

Add `--prompt` to send a question and wait for the answer:

```bash
answer=$(telegram-notify --text "Which environment should I deploy to?" --prompt)
```

The command waits up to five minutes for a plain-text reply and prints the
reply on stdout. The bot reacts with 👀 to the reply.

Add preset answers with repeatable `--button` flags:

```bash
answer=$(telegram-notify --text "Proceed with deploy?" --prompt \
  --button Yes --button No)
```

A button tap removes the buttons, appends `Answer: <label>` to the question,
reacts with 👍, and prints the label on stdout. A text reply still works when
buttons are present.

About 30 seconds before the deadline, the bot sends a check-in message. React
to it with any emoji to add five minutes. This can repeat. On timeout, the bot
removes the buttons, sends `Request timed out waiting for a reply.`, writes an
error to stderr, and exits nonzero without writing to stdout.

Prompt mode requires a numeric private or group chat ID. Channel usernames are
valid send targets but cannot receive prompts.

## Override or replace settings

Environment variables are the weakest source, saved config overrides them, and
command-line overrides are strongest:

```bash
TG_BOT_TOKEN=temporary TG_CHAT_ID=42 telegram-notify --text "Hello"
telegram-notify --token temporary --chat-id 42 --text "Hello"
```

`TG_CHAT_ID` and `--chat-id` accept a nonzero numeric ID or an `@channel`
username.

Save new values without contacting Telegram:

```bash
telegram-notify --set-token TOKEN
telegram-notify --set-chat-id 42
telegram-notify --set-token TOKEN --set-chat-id @alerts
```

These flags update only the fields you provide. `--set-token` checks only that
the value is not empty. `--set-chat-id` checks its format. Passing a token on
the command line can leave it in shell history and the process list.

To replace the saved private chat through Telegram, run:

```bash
telegram-notify --learn
```

Then send `/start` to the bot. Learning and prompt modes use Telegram long
polling and cannot run while the bot has an active webhook. Press Ctrl+C to
stop waiting.
