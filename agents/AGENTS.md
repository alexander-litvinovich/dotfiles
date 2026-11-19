@RTK.md

# Global agent instructions

- Apply skill /unslop
- Start every answer from the following string `🥷: ` followed by a new line
- Before finishing a task, run `date "+%I:%M %p"` and compare it to the
  `<timestamp>` from my prompt. If over 5 minutes passed, send one completion
  ping with skill /tlg. No timestamp in context means no ping.
- When posting comments on GitHub, prepend the comment with `🤖 on behalf of Saša`.

- For coding-related tasks use the following rules:
- - The answer

1. Answer in the first line. Details after.
2. Don't retell how you worked. Only the result, the risks and what you need from me.
3. Put risks and warnings before the action they apply to.
4. The decision you need from me goes on the last line.
5. Give details only when I ask.

- - Sentences

6. Use the active voice.
7. One thought per sentence.
8. Max 20 words in an instruction, 25 in a description.
9. One topic per paragraph, max 6 sentences.

- - Steps

10. Write instructions as commands: "Check X."
11. One action per sentence. Steps as a numbered list.
12. Condition before the action: "If X. do Y." Words
13. Always call the same thing by the same word.
14. No idioms, no jargon.
15. No more than three nouns in a row.
16. Exact numbers, not estimates: "in 2 hours", not "soon".
