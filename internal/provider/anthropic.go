package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/meeseeks/koko/internal/httputil"
)

type AnthropicProvider struct {
	apiKey    string
	model     string
	maxTokens int
	client    *http.Client
}

func NewAnthropic(apiKey, model string, maxTokens int) (*AnthropicProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}
	if maxTokens <= 0 {
		maxTokens = 16384
	}
	return &AnthropicProvider{
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		client:    &http.Client{},
	}, nil
}

func (a *AnthropicProvider) Name() string      { return "anthropic" }
func (a *AnthropicProvider) Model() string     { return a.model }
func (a *AnthropicProvider) SetModel(m string) { a.model = m }

func (a *AnthropicProvider) buildHTTPRequest(ctx context.Context, messages []Message, tools []ToolDef, stream bool) (*http.Request, error) {
	var system string
	var apiMessages []anthropicMessage
	for _, m := range messages {
		if m.Role == RoleSystem {
			system = m.Content
			continue
		}
		if len(m.Images) > 0 {
			var blocks []map[string]interface{}
			for _, img := range m.Images {
				blocks = append(blocks, map[string]interface{}{
					"type": "image",
					"source": map[string]interface{}{
						"type":       "base64",
						"media_type": img.MimeType,
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
			apiMessages = append(apiMessages, anthropicMessage{
				Role:    string(m.Role),
				Content: blocks,
			})
		} else {
			apiMessages = append(apiMessages, anthropicMessage{
				Role:    string(m.Role),
				Content: m.Content,
			})
		}
	}

	reqBody := anthropicRequest{
		Model:     a.model,
		MaxTokens: a.maxTokens,
		System:    system,
		Messages:  apiMessages,
		Stream:    stream,
	}

	if len(tools) > 0 {
		for _, t := range tools {
			reqBody.Tools = append(reqBody.Tools, anthropicTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.Parameters,
			})
		}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}
	return req, nil
}

func (a *AnthropicProvider) Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	req, err := a.buildHTTPRequest(ctx, messages, tools, false)
	if err != nil {
		return nil, err
	}

	resp, err := httputil.DoWithRetry(ctx, a.client, req, 5)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, sanitizeErrorBody(respBody, 512))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	result := &Response{
		Usage: Usage{
			InputTokens:  apiResp.Usage.InputTokens,
			OutputTokens: apiResp.Usage.OutputTokens,
		},
	}
	for _, block := range apiResp.Content {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "tool_use":
			args := make(map[string]string)
			for k, v := range block.Input {
				args[k] = fmt.Sprintf("%v", v)
			}
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				Name: block.Name,
				Args: args,
			})
		}
	}

	return result, nil
}

func (a *AnthropicProvider) ChatStream(ctx context.Context, messages []Message, tools []ToolDef, onDelta func(StreamDelta)) (*Response, error) {
	req, err := a.buildHTTPRequest(ctx, messages, tools, true)
	if err != nil {
		return nil, err
	}

	resp, err := httputil.DoWithRetry(ctx, a.client, req, 5)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, sanitizeErrorBody(respBody, 512))
	}

	result := &Response{}
	var currentToolName string
	var currentToolInput strings.Builder

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			if event.Message.Usage.InputTokens > 0 {
				result.Usage.InputTokens = event.Message.Usage.InputTokens
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
				var raw map[string]interface{}
				if err := json.Unmarshal([]byte(currentToolInput.String()), &raw); err == nil {
					for k, v := range raw {
						args[k] = fmt.Sprintf("%v", v)
					}
				}
				result.ToolCalls = append(result.ToolCalls, ToolCall{
					Name: currentToolName,
					Args: args,
				})
				currentToolName = ""
			}
		case "message_delta":
			if event.Usage.OutputTokens > 0 {
				result.Usage.OutputTokens = event.Usage.OutputTokens
			}
		case "message_stop":
			if onDelta != nil {
				onDelta(StreamDelta{Done: true, Response: result})
			}
		}
	}

	return result, nil
}

type anthropicStreamEvent struct {
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
	Message struct {
		Usage anthropicUsage `json:"usage"`
	} `json:"message,omitempty"`
	Usage anthropicUsage `json:"usage,omitempty"`
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicContentBlock struct {
	Type  string                 `json:"type"`
	Text  string                 `json:"text,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Usage   anthropicUsage          `json:"usage"`
}
