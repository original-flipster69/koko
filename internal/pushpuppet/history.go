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
	p.history = p.history[:1]
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
	p.lastInputTokens = 0
	newTokens := p.measureTokens(context.Background())
	p.lastInputTokens = newTokens
	return oldTokens, newTokens
}

func (p *PushPuppet) trimHistory(ctx context.Context) {
	realTotal := p.estimateTokens()
	if realTotal <= maxHistoryTokens {
		return
	}
	target := maxHistoryTokens * 3 / 4

	scale := 1.0
	if estTotal := estimateMessagesTokens(p.history); estTotal > 0 {
		scale = float64(realTotal) / float64(estTotal)
	}

	cutEnd := 0
	droppedReal := 0.0
	for i := 2; i <= len(p.history)-2; i++ {
		droppedReal += float64(msgTokens(p.history[i-1])) * scale
		if p.history[i].Role != provider.User {
			continue
		}
		cutEnd = i
		if float64(realTotal)-droppedReal <= float64(target) {
			break
		}
	}

	if cutEnd == 0 {
		return
	}

	systemMsg := p.history[0]
	dropped := p.history[1:cutEnd]
	summary := summarizeMessages(dropped)
	kept := p.history[cutEnd:]

	newHist := make([]provider.Msg, 0, len(kept)+3)
	newHist = append(newHist, systemMsg)
	newHist = append(newHist, provider.Msg{Role: provider.User, Content: summary})
	newHist = append(newHist, provider.Msg{Role: provider.Assistant, Content: trimAck})
	newHist = append(newHist, kept...)
	p.history = newHist
	p.lastInputTokens = p.measureTokens(ctx)
	slog.Info("history trimmed with summary", "dropped_messages", len(dropped), "kept_messages", len(kept), "real_before", realTotal, "scale", scale)
}

func (p *PushPuppet) estimateTokens() int {
	if p.lastInputTokens > 0 {
		return p.lastInputTokens
	}
	return estimateMessagesTokens(p.history)
}

func estimateMessagesTokens(msgs []provider.Msg) int {
	total := 0
	for _, m := range msgs {
		total += msgTokens(m)
	}
	return total
}

func msgTokens(m provider.Msg) int {
	return len([]rune(m.Content))*10/35 + 4
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
		if m.Role == provider.User {
			if strings.HasPrefix(m.Content, "Previous conversation") {
				extractSummaryFacts(m.Content, &filesModified, &filesRead, &commandsRun, &errors, &userRequests)
				continue
			}
			if strings.Contains(m.Content, "<tool_output") {
				extractToolOutputFacts(m.Content, &filesModified, &filesRead, &commandsRun, &errors)
				continue
			}
			req := m.Content
			if len(req) > maxSummarizedRequestLen {
				req = req[:maxSummarizedRequestLen] + "..."
			}
			userRequests = append(userRequests, req)
			continue
		}
		extractToolOutputFacts(m.Content, &filesModified, &filesRead, &commandsRun, &errors)
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
