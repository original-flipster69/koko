package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

type claude struct {
	apiKey    string
	model     string
	baseURL   string
	maxTokens int
	effort    Effort
	client    *http.Client
}

func newClaude(apiKey, model, baseURL string, maxTokens int) (*claude, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("CLAUDE_API_KEY not set")
	}
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	if maxTokens <= 0 {
		maxTokens = 16384
	}
	return &claude{
		apiKey:    apiKey,
		model:     model,
		baseURL:   baseURL,
		maxTokens: maxTokens,
		client:    &http.Client{},
	}, nil
}

var ephemeral = map[string]string{"type": "ephemeral"}

var _ TokenCounter = (*claude)(nil)

func (a *claude) Name() string       { return "claude" }
func (a *claude) Model() string      { return a.model }
func (a *claude) SetModel(m string)  { a.model = m }
func (a *claude) Effort() Effort     { return a.effort }
func (a *claude) SetEffort(e Effort) { a.effort = e }

func (a *claude) request(msgs []Msg, tools []ToolDef, stream bool) claudeReq {
	msgs = flattenToolMessages(msgs)
	var system string
	var apiMessages []claudeMsg
	for _, m := range msgs {
		if m.Role == System {
			system = m.Content
			continue
		}
		if len(m.Imgs) > 0 {
			var blocks []map[string]interface{}
			for _, img := range m.Imgs {
				blocks = append(blocks, map[string]interface{}{
					"type": "image",
					"source": map[string]interface{}{
						"type":       "base64",
						"media_type": img.Mime,
						"data":       img.Data,
					},
				})
			}
			if m.Content != "" {
				blocks = append(blocks, map[string]interface{}{
					"type": "text",
					"text": m.Content,
				})
			}
			apiMessages = append(apiMessages, claudeMsg{
				Role:    string(m.Role),
				Content: blocks,
			})
		} else {
			apiMessages = append(apiMessages, claudeMsg{
				Role:    string(m.Role),
				Content: m.Content,
			})
		}
	}

	reqBody := claudeReq{
		Model:     a.model,
		MaxTokens: a.maxTokens,
		Msgs:      apiMessages,
		Stream:    stream,
	}
	if system != "" {
		reqBody.System = []map[string]interface{}{
			{"type": "text", "text": system, "cache_control": ephemeral},
		}
	}
	for _, t := range tools {
		reqBody.Tools = append(reqBody.Tools, claudeTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Params,
		})
	}
	if a.effort != EffortDefault {
		reqBody.Thinking = &claudeThinking{Type: "adaptive"}
		reqBody.OutputConfig = &claudeOutputConfig{Effort: string(a.effort)}
	}
	markCacheBreakpoints(&reqBody)
	return reqBody
}

func markCacheBreakpoints(reqBody *claudeReq) {
	if n := len(reqBody.Tools); n > 0 {
		reqBody.Tools[n-1].CacheControl = ephemeral
	}
	if n := len(reqBody.Msgs); n > 0 {
		last := &reqBody.Msgs[n-1]
		switch c := last.Content.(type) {
		case string:
			last.Content = []map[string]interface{}{
				{"type": "text", "text": c, "cache_control": ephemeral},
			}
		case []map[string]interface{}:
			if len(c) > 0 {
				c[len(c)-1]["cache_control"] = ephemeral
			}
		}
	}
}

func (a *claude) headers() map[string]string {
	return map[string]string{
		"x-api-key":         a.apiKey,
		"anthropic-version": "2023-06-01",
	}
}

func (a *claude) CountTokens(ctx context.Context, msgs []Msg, tools []ToolDef) (int, error) {
	r := a.request(msgs, tools, false)
	body := claudeCountReq{Model: r.Model, System: r.System, Msgs: r.Msgs, Tools: r.Tools}
	resp, err := sendReq(ctx, a.client, a.baseURL+"/messages/count_tokens", body, a.headers())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var out struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, fmt.Errorf("parsing token count: %w", err)
	}
	return out.InputTokens, nil
}

func (a *claude) ChatStream(ctx context.Context, msgs []Msg, tools []ToolDef, onDelta func(StreamDelta)) (*Response, error) {
	streamCtx, guard := newStallGuard(ctx, streamIdleTimeout)
	defer guard.done()

	resp, err := sendReq(streamCtx, a.client, a.baseURL+"/messages", a.request(msgs, tools, true), a.headers())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result := &Response{}
	var currentToolName string
	var currentToolInput strings.Builder

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		guard.progress()
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event claudeStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			u := event.Msg.Usage
			if in := u.Input + u.CacheCreation + u.CacheRead; in > 0 {
				result.Usage.InputTokens = in
			}
			if u.CacheRead > 0 || u.CacheCreation > 0 {
				slog.Debug("claude prompt cache", "read", u.CacheRead, "write", u.CacheCreation, "uncached", u.Input)
			}
		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				currentToolName = event.ContentBlock.Name
				currentToolInput.Reset()
			}
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				result.Content += event.Delta.Text
				if onDelta != nil {
					onDelta(StreamDelta{Text: event.Delta.Text})
				}
			} else if event.Delta.Type == "input_json_delta" {
				currentToolInput.WriteString(event.Delta.PartialJSON)
			}
		case "content_block_stop":
			if currentToolName != "" {
				args := make(map[string]string)
				rawArgs := currentToolInput.String()
				var raw map[string]interface{}
				if err := json.Unmarshal([]byte(rawArgs), &raw); err == nil {
					args = coerceArgs(raw)
				} else {
					slog.Warn("claude stream tool args parse failed", "tool", currentToolName, "raw_len", len(rawArgs), "err", err)
				}
				result.ToolCalls = append(result.ToolCalls, ToolCall{
					Name: currentToolName,
					Args: args,
				})
				currentToolName = ""
			}
		case "message_delta":
			if event.Usg.Output > 0 {
				result.Usage.OutputTokens = event.Usg.Output
			}
			if event.Delta.StopReason != "" {
				result.StopReason = event.Delta.StopReason
			}
		case "message_stop":
			if onDelta != nil {
				onDelta(StreamDelta{Done: true, Response: result})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading stream: %w", streamErr(streamCtx, err))
	}

	return result, nil
}

type claudeStreamEvent struct {
	Type         string `json:"type"`
	ContentBlock struct {
		Type string `json:"type"`
		Name string `json:"name,omitempty"`
	} `json:"content_block,omitempty"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
		StopReason  string `json:"stop_reason,omitempty"`
	} `json:"delta,omitempty"`
	Msg struct {
		Usage claudeUsg `json:"usage"`
	} `json:"message,omitempty"`
	Usg claudeUsg `json:"usage,omitempty"`
}

type claudeMsg struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type claudeTool struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	InputSchema  Schema            `json:"input_schema"`
	CacheControl map[string]string `json:"cache_control,omitempty"`
}

type claudeReq struct {
	Model        string              `json:"model"`
	MaxTokens    int                 `json:"max_tokens"`
	System       interface{}         `json:"system,omitempty"`
	Msgs         []claudeMsg         `json:"messages"`
	Tools        []claudeTool        `json:"tools,omitempty"`
	Stream       bool                `json:"stream,omitempty"`
	Thinking     *claudeThinking     `json:"thinking,omitempty"`
	OutputConfig *claudeOutputConfig `json:"output_config,omitempty"`
}

type claudeThinking struct {
	Type string `json:"type"`
}

type claudeOutputConfig struct {
	Effort string `json:"effort,omitempty"`
}

type claudeCountReq struct {
	Model  string       `json:"model"`
	System interface{}  `json:"system,omitempty"`
	Msgs   []claudeMsg  `json:"messages"`
	Tools  []claudeTool `json:"tools,omitempty"`
}

type claudeUsg struct {
	Input         int `json:"input_tokens"`
	Output        int `json:"output_tokens"`
	CacheCreation int `json:"cache_creation_input_tokens"`
	CacheRead     int `json:"cache_read_input_tokens"`
}
