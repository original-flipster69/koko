package provider

type TokenEstimator func([]Msg) int

func MsgTokens(m Msg) int {
	return claudeMsgTokens(m)
}

func claudeMsgTokens(m Msg) int {
	n := len([]rune(m.Content))
	for _, tc := range m.ToolCalls {
		n += len(tc.Name)
		for k, v := range tc.Args {
			n += len(k) + len([]rune(v))
		}
	}
	return (n * 4) / 3
}

func EstimateClaudeTokens(msgs []Msg) int {
	total := 0
	for _, m := range msgs {
		total += claudeMsgTokens(m)
	}
	return total
}

func EstimateMistralTokens(msgs []Msg) int {
	return EstimateClaudeTokens(msgs)
}

func EstimateOllamaTokens(msgs []Msg) int {
	return EstimateClaudeTokens(msgs)
}

func EstimateMessagesTokens(msgs []Msg) int {
	return EstimateClaudeTokens(msgs)
}

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
