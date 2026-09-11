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
