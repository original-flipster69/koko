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

func (a *claude) Name() string      { return "claude" }
func (a *claude) Model() string     { return a.model }
func (a *claude) SetModel(m string) { a.model = m }

func (a *claude) request(msgs []Msg, tools []ToolDef, stream bool) claudeReq {
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
		System:    system,
		Msgs:      apiMessages,
		Stream:    stream,
	}
	for _, t := range tools {
		reqBody.Tools = append(reqBody.Tools, claudeTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Params,
		})
	}
	return reqBody
}

func (a *claude) ChatStream(ctx context.Context, msgs []Msg, tools []ToolDef, onDelta func(StreamDelta)) (*Response, error) {
	resp, err := sendReq(ctx, a.client, a.baseURL+"/msgs", a.request(msgs, tools, true), map[string]string{
		"x-api-key":      a.apiKey,
		"claude-version": "2023-06-01",
	})
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
			if event.Msg.Usage.Input > 0 {
				result.Usage.InputTokens = event.Msg.Usage.Input
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
					slog.Warn("claude stream tool args parse failed", "tool", currentToolName, "raw", rawArgs, "err", err)
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
		case "message_stop":
			if onDelta != nil {
				onDelta(StreamDelta{Done: true, Response: result})
			}
		}
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
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type claudeReq struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Msgs      []claudeMsg  `json:"messages"`
	Tools     []claudeTool `json:"tools,omitempty"`
	Stream    bool         `json:"stream,omitempty"`
}

type claudeUsg struct {
	Input  int `json:"input_tokens"`
	Output int `json:"output"`
}
