package provider

type TokenEstimator func([]Msg) int

func MsgTokens(m Msg) int {
	return msgTokens(m)
}

func msgTokens(m Msg) int {
	n := len([]rune(m.Content))
	for _, tc := range m.ToolCalls {
		n += len(tc.Name)
		for k, v := range tc.Args {
			n += len(k) + len([]rune(v))
		}
	}
	return (n * 4) / 3
}

func EstimateTokens(msgs []Msg) int {
	total := 0
	for _, m := range msgs {
		total += msgTokens(m)
	}
	return total
}
