# fastgit
agentic git commit generate tool

## Command Overview
- `fastgit commit`: AI 提交流程（保留原行为）
- `fastgit commit ai`: AI 提交流程显式入口（新增）
- `fastgit ggc list`: 查看统一命令面（ggc 风格）
- `fastgit ggc <command ...>`: 执行统一命令，例如 `fastgit ggc status short`

## New ggc-style command surface (phase 1)

- `fastgit ggc status|status short`
- `fastgit ggc add <file|.>`
- `fastgit ggc commit <message>`
- `fastgit ggc log simple|graph`
- `fastgit ggc diff|diff staged|diff unstaged`
- `fastgit ggc branch current|list local|list remote|checkout <name>|checkout remote <name>|create <name>|delete <name>`
- `fastgit ggc fetch|fetch prune`
- `fastgit ggc pull current|pull rebase`
- `fastgit ggc push current|push force`
- `fastgit ggc rebase <upstream>|continue|abort|skip`
- `fastgit ggc tag list|show <tag>`
- `fastgit ggc remote list`

## Refer
- https://github.com/Nutlope/aicommits
- https://chat.deepseek.com

## ENV
- OPENAI_API_KEY
- OPENAI_BASE_URL, default: https://api.deepseek.com/v1
- OPENAI_MODEL, default: deepseek-chat
