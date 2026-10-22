package provider

// TokenEstimator is a function type for estimating the number of tokens in a list of messages.
type TokenEstimator func([]Msg) int

// MsgTokens estimates tokens for a single message using a generic heuristic.
// This is a convenience function for backward compatibility.
func MsgTokens(m Msg) int {
	return claudeMsgTokens(m)
}

// claudeMsgTokens estimates tokens for a single message using Claude's tokenizer heuristic.
// Claude's tokenizer: ~4 tokens per 3 characters (empirical).
func claudeMsgTokens(m Msg) int {
	n := len([]rune(m.Content))
	for _, tc := range m.ToolCalls {
		n += len(tc.Name)
		for k, v := range tc.Args {
			n += len(k) + len([]rune(v))
		}
	}
	// Claude's tokenizer: ~4 tokens per 3 characters (empirical)
	return (n * 4) / 3
}

// EstimateClaudeTokens uses a more accurate heuristic for Claude models (~4 tokens per 3 characters).
func EstimateClaudeTokens(msgs []Msg) int {
	total := 0
	for _, m := range msgs {
		total += claudeMsgTokens(m)
	}
	return total
}

// EstimateMistralTokens uses a heuristic for Mistral models (similar to Claude).
func EstimateMistralTokens(msgs []Msg) int {
	return EstimateClaudeTokens(msgs) // Mistral's tokenizer is similar to Claude's
}

// EstimateOllamaTokens uses a heuristic for Ollama models (similar to Claude).
func EstimateOllamaTokens(msgs []Msg) int {
	return EstimateClaudeTokens(msgs) // Ollama's tokenizer is similar for most models
}

// EstimateMessagesTokens is a generic token estimator for messages.
// It uses the same heuristic as Claude for backward compatibility.
func EstimateMessagesTokens(msgs []Msg) int {
	return EstimateClaudeTokens(msgs)
}

// GetTokenEstimator returns the appropriate token estimator for the given provider name.
func GetTokenEstimator(providerName string) TokenEstimator {
	switch providerName {
	case "claude":
		return EstimateClaudeTokens
	case "mistral":
		return EstimateMistralTokens
	case "ollama":
		return EstimateOllamaTokens
	default:
		return EstimateMessagesTokens
	}
}
