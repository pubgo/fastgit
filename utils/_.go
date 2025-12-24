package utils

// https://github.com/gogs/git-module/blob/master/repo.go
// https://github.com/sabhiram/go-gitignore
// github.com/kevinburke/ssh_config
// https://github.com/moby/moby/blob/master/pkg/tailfile/tailfile.go
// https://github.com/cli/cli/blob/trunk/git/client.go
// https://github.com/MakeNowJust/heredoc
// https://github.com/gohugoio/hugo/blob/master/watcher/filenotify/filenotify.go
// https://github.com/itchyny/gojo Yet another Go implementation of jo
// github.com/itchyny/gojq
// https://github.com/bobheadxi/streamline
// https://github.com/coder/serpent/blob/main/serpent.go
// https://github.com/coder/serpent/blob/main/command.go
// https://github.com/WireGuard/wireguard-go
// https://github.com/WireGuard/wgctrl-go
// https://github.com/coder/wgtunnel
// github.com/tiktoken-go/tokenizer
func CountTokens(msgs ...openai.ChatCompletionMessage) int {
	enc, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		panic("oh oh")
	}

	var tokens int
	for _, msg := range msgs {
		ts, _, _ := enc.Encode(msg.Content)
		tokens += len(ts)

		for _, call := range msg.ToolCalls {
			ts, _, _ = enc.Encode(call.Function.Arguments)
			tokens += len(ts)
		}
	}
	return tokens
}

func Ellipse(s string, maxTokens int) string {
	enc, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		panic("failed to get tokenizer")
	}

	tokens, _, _ := enc.Encode(s)
	if len(tokens) <= maxTokens {
		return s
	}

	// Decode the truncated tokens back to a string
	truncated, _ := enc.Decode(tokens[:maxTokens])
	return truncated + "..."
}

// https://github.com/coder/aicommit/blob/main/prompt.go
// https://github.com/coder/wush/blob/main/cmd/wush/main.go
