package pushpuppet

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/original-flipster69/koko/internal/provider"
)

const (
	maxHistoryTokens        = 100000
	maxToolResultSize       = 10240
	maxSummarizedRequestLen = 150
	compactAck              = "Understood. I have the context from our previous conversation. How can I help?"
	trimAck                 = "Understood, continuing with this context."
	summaryHeader           = "Previous conversation context:\n\n"
)

func (p *PushPuppet) ClearHistory() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.history) > 0 {
		p.history = p.history[:1]
	}
	p.TotalInput = 0
	p.TotalOutput = 0
	p.lastInputTokens = 0
}

func (p *PushPuppet) HistoryLen() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.history) - 1
}

func (p *PushPuppet) SaveSession(dir string) error {
	p.mu.Lock()
	snapshot := append([]provider.Msg(nil), p.history[1:]...)
	p.mu.Unlock()
	return saveSession(dir, snapshot)
}

func (p *PushPuppet) LoadSession(dir string) error {
	msgs, err := loadSession(dir)
	if err != nil {
		return err
	}
	for len(msgs) > 0 && msgs[0].Role == provider.System {
		msgs = msgs[1:]
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	newHist := make([]provider.Msg, 0, 1+len(msgs))
	newHist = append(newHist, p.history[0])
	newHist = append(newHist, msgs...)
	p.history = newHist
	p.TotalInput = estimateMessagesTokens(p.history)
	p.TotalOutput = 0
	p.lastInputTokens = p.TotalInput
	return nil
}

func (p *PushPuppet) Compact() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.history) <= 2 {
		return 0, 0
	}
	oldTokens := p.estimateTokens()
	summary := summarizeMessages(p.history[1:])
	p.history = []provider.Msg{
		p.history[0],
		{Role: provider.User, Content: summary},
		{Role: provider.Assistant, Content: compactAck},
	}
	if p.counter != nil {
		p.lastInputTokens = p.measureTokens(context.Background())
	} else {
		p.lastInputTokens = estimateMessagesTokens(p.history)
	}
	p.TotalInput = p.lastInputTokens
	return oldTokens, p.lastInputTokens
}

func (p *PushPuppet) trimHistory(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	realTotal := p.lastInputTokens
	if realTotal == 0 {
		realTotal = estimateMessagesTokens(p.history)
	}

	if realTotal <= maxHistoryTokens {
		return
	}

	target := maxHistoryTokens * 3 / 5

	tokensToDrop := realTotal - target
	if tokensToDrop <= 0 {
		return
	}

	newHist := make([]provider.Msg, 0, len(p.history))
	newHist = append(newHist, p.history[0])
	droppedTokens := 0

	for i := 1; i < len(p.history); i++ {
		msg := p.history[i]
		msgTokenEstimate := provider.MsgTokens(msg)
		if droppedTokens+msgTokenEstimate > tokensToDrop {
			newHist = append(newHist, p.history[i:]...)
			break
		} else {
			droppedTokens += msgTokenEstimate
		}
	}

	if len(newHist) < len(p.history) {
		dropped := p.history[1 : len(p.history)-len(newHist)+1]
		summary := summarizeMessages(dropped)
		newHistWithSummary := make([]provider.Msg, 0, len(newHist)+2)
		newHistWithSummary = append(newHistWithSummary, newHist[0])
		newHistWithSummary = append(newHistWithSummary, provider.Msg{
			Role:    provider.User,
			Content: summary,
		})
		newHistWithSummary = append(newHistWithSummary, provider.Msg{
			Role:    provider.Assistant,
			Content: trimAck,
		})
		newHistWithSummary = append(newHistWithSummary, newHist[1:]...)
		p.history = newHistWithSummary
	} else {
		p.history = newHist
	}

	p.lastInputTokens = 0
	if p.counter != nil {
		p.lastInputTokens = p.measureTokens(ctx)
		p.TotalInput = p.lastInputTokens
	} else {
		p.TotalInput = estimateMessagesTokens(p.history)
	}
	slog.Info("history trimmed", "dropped_tokens", droppedTokens, "real_before", realTotal, "target", target)
}

func (p *PushPuppet) estimateTokens() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lastInputTokens > 0 {
		return p.lastInputTokens
	}
	if p.tokenEstimator != nil {
		return p.tokenEstimator(p.history)
	}
	return provider.EstimateTokens(p.history)
}

func estimateMessagesTokens(msgs []provider.Msg) int {
	return provider.EstimateTokens(msgs)
}

func truncateForHistory(result string) string {
	if len(result) <= maxToolResultSize {
		return result
	}
	return result[:maxToolResultSize] + "\n...(truncated)"
}

func summarizeMessages(msgs []provider.Msg) string {
	var out strings.Builder
	out.WriteString(summaryHeader)

	var filesModified []string
	var filesRead []string
	var commandsRun []string
	var errors []string
	var userRequests []string

	for _, m := range msgs {
		switch m.Role {
		case provider.User:
			if strings.HasPrefix(m.Content, "Previous conversation") {
				extractSummaryFacts(m.Content, &filesModified, &filesRead, &commandsRun, &errors, &userRequests)
				continue
			}
			req := m.Content
			if req == "" {
				continue
			}
			if len(req) > maxSummarizedRequestLen {
				req = req[:maxSummarizedRequestLen] + "..."
			}
			userRequests = append(userRequests, req)
		case provider.Assistant:
			for _, tc := range m.ToolCalls {
				switch tc.Name {
				case "write_file", "replace_in_file", "delete_file":
					if path := tc.Args["path"]; path != "" {
						filesModified = append(filesModified, path)
					}
				case "rename_file":
					if path := tc.Args["new_path"]; path != "" {
						filesModified = append(filesModified, path)
					}
				case "read_file":
					if path := tc.Args["path"]; path != "" {
						filesRead = append(filesRead, path)
					}
				case "exec_command":
					cmd := tc.Args["command"]
					if cmd == "" {
						cmd = "(command)"
					}
					commandsRun = append(commandsRun, cmd)
				}
			}
		case provider.Tool:
			line := m.Content
			if i := strings.IndexByte(line, '\n'); i >= 0 {
				line = line[:i]
			}
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "error:") && len(line) < 200 {
				errors = append(errors, line)
			}
		}
	}

	if len(userRequests) > 0 {
		out.WriteString("User requests:\n")
		for _, r := range dedupe(userRequests) {
			out.WriteString("- " + r + "\n")
		}
		out.WriteString("\n")
	}
	if len(filesModified) > 0 {
		out.WriteString("Files modified: " + strings.Join(dedupePaths(filesModified), ", ") + "\n")
	}
	if len(filesRead) > 0 {
		out.WriteString("Files read: " + strings.Join(dedupePaths(filesRead), ", ") + "\n")
	}
	if len(commandsRun) > 0 {
		out.WriteString("Commands executed: " + strings.Join(dedupe(commandsRun), ", ") + "\n")
	}
	if len(errors) > 0 {
		out.WriteString("\nErrors encountered:\n")
		for _, e := range dedupe(errors) {
			out.WriteString("- " + e + "\n")
		}
	}
	return out.String()
}

func extractToolOutputFacts(content string, modified, read, commands, errors *[]string) {
	var currentTool string
	var awaitingFirstContent bool
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)

		if currentTool == "" {
			if strings.HasPrefix(line, "<tool_output name=") {
				if a := strings.Index(line, `"`); a >= 0 {
					if b := strings.Index(line[a+1:], `"`); b > 0 {
						currentTool = line[a+1 : a+1+b]
						awaitingFirstContent = true
					}
				}
			}
		} else {
			if line == "</tool_output>" {
				currentTool = ""
				awaitingFirstContent = false
			} else if awaitingFirstContent && line != "" {
				awaitingFirstContent = false
				path := extractPathFromContentLine(line)
				switch currentTool {
				case "replace_in_file", "write_file", "delete_file", "rename_file":
					if path != "" {
						*modified = append(*modified, path)
					}
				case "read_file":
					if path != "" {
						*read = append(*read, path)
					}
				case "exec_command":
					*commands = append(*commands, "(command)")
				}
			}
		}

		if strings.Contains(line, "error:") && len(line) < 200 {
			*errors = append(*errors, line)
		}
	}
}

func extractSummaryFacts(content string, modified, read, commands, errors, userRequests *[]string) {
	const (
		secNone     = ""
		secRequests = "userRequests"
		secErrors   = "errors"
	)
	section := secNone
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Files modified: "):
			section = secNone
			for _, p := range strings.Split(strings.TrimPrefix(line, "Files modified: "), ", ") {
				if p = strings.TrimSpace(p); p != "" {
					*modified = append(*modified, p)
				}
			}
		case strings.HasPrefix(line, "Files read: "):
			section = secNone
			for _, p := range strings.Split(strings.TrimPrefix(line, "Files read: "), ", ") {
				if p = strings.TrimSpace(p); p != "" {
					*read = append(*read, p)
				}
			}
		case strings.HasPrefix(line, "Commands executed: "):
			section = secNone
			for _, c := range strings.Split(strings.TrimPrefix(line, "Commands executed: "), ", ") {
				if c = strings.TrimSpace(c); c != "" {
					*commands = append(*commands, c)
				}
			}
		case line == "User requests:":
			section = secRequests
		case line == "Errors encountered:":
			section = secErrors
		case line == "":
			section = secNone
		case strings.HasPrefix(line, "- "):
			v := strings.TrimPrefix(line, "- ")
			switch section {
			case secRequests:
				*userRequests = append(*userRequests, v)
			case secErrors:
				*errors = append(*errors, v)
			}
		}
	}
}

func extractPathFromContentLine(line string) string {
	for _, prefix := range []string{"updated ", "wrote ", "deleted ", "renamed ", "["} {
		if strings.HasPrefix(line, prefix) {
			path := strings.TrimPrefix(line, prefix)
			if idx := strings.IndexAny(path, " \n]"); idx > 0 {
				path = path[:idx]
			}
			return path
		}
	}
	return ""
}

func dedupe(s []string) []string {
	seen := make(map[string]bool, len(s))
	out := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func dedupePaths(s []string) []string {
	seen := make(map[string]bool, len(s))
	out := make([]string, 0, len(s))
	for _, v := range s {
		key := filepath.Clean(v)
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}
