@RTK.md

# Global agent instructions

- Apply skill /unslop
- Start every answer from the following string `🥷: ` followed by a new line
- Before finishing a task, run `date "+%I:%M %p"` and compare it to the
  `<timestamp>` from my prompt. If over 5 minutes passed, send one completion
  ping with skill /tlg. No timestamp in context means no ping.
- When posting comments on GitHub, prepend the comment with `🤖 on behalf of Saša`.
