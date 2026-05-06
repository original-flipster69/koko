package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/meeseeks/koko/internal/httputil"
)

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type MistralProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func NewMistral(apiKey, model, baseURL string) (*MistralProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("MISTRAL_API_KEY not set")
	}
	if baseURL == "" {
		baseURL = "https://api.mistral.ai/v1"
	}
	return &MistralProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{},
	}, nil
}

func (m *MistralProvider) Name() string      { return "mistral" }
func (m *MistralProvider) Model() string     { return m.model }
func (m *MistralProvider) SetModel(s string) { m.model = s }

func toMistralMessages(messages []Message) []mistralMessage {
	var out []mistralMessage
	for _, msg := range messages {
		if len(msg.Images) > 0 {
			var blocks []map[string]interface{}
			for _, img := range msg.Images {
				blocks = append(blocks, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]string{
						"url": "data:" + img.MimeType + ";base64," + img.Data,
					},
				})
			}
			if msg.Content != "" {
				blocks = append(blocks, map[string]interface{}{
					"type": "text",
					"text": msg.Content,
				})
			}
			out = append(out, mistralMessage{Role: string(msg.Role), Content: blocks})
		} else {
			out = append(out, mistralMessage{Role: string(msg.Role), Content: msg.Content})
		}
	}
	return out
}

func (m *MistralProvider) ChatStream(ctx context.Context, messages []Message, tools []ToolDef, onDelta func(StreamDelta)) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	mistralMessages := toMistralMessages(messages)

	reqBody := mistralRequest{
		Model:    m.model,
		Messages: mistralMessages,
		Stream:   true,
	}
	if len(tools) > 0 {
		for _, t := range tools {
			reqBody.Tools = append(reqBody.Tools, mistralTool{
				Type: "function",
				Function: mistralFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			})
		}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}

	resp, err := httputil.DoWithRetry(ctx, m.client, req, 5)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, sanitizeErrorBody(body, 512))
	}

	type toolAcc struct {
		name string
		args strings.Builder
	}

	var content strings.Builder
	var usage Usage
	toolAccs := make(map[int]*toolAcc)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var chunk mistralStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				content.WriteString(delta.Content)
				if onDelta != nil {
					onDelta(StreamDelta{Text: delta.Content})
				}
			}
			for _, tc := range delta.ToolCalls {
				acc, ok := toolAccs[tc.Index]
				if !ok {
					acc = &toolAcc{}
					toolAccs[tc.Index] = acc
				}
				if tc.Function.Name != "" {
					acc.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					acc.args.WriteString(tc.Function.Arguments)
				}
			}
		}
		if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
			usage = Usage{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading stream: %w", err)
	}

	result := &Response{Content: content.String(), Usage: usage}

	for i := 0; i < len(toolAccs); i++ {
		acc, ok := toolAccs[i]
		if !ok {
			continue
		}
		rawArgs := acc.args.String()
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(rawArgs), &parsed); err != nil {
			slog.Warn("mistral stream tool args parse failed", "tool", acc.name, "raw", rawArgs, "err", err)
			continue
		}
		args := make(map[string]string, len(parsed))
		for k, v := range parsed {
			if v == nil {
				continue
			}
			switch vv := v.(type) {
			case string:
				args[k] = vv
			case float64:
				args[k] = strconv.FormatFloat(vv, 'f', -1, 64)
			case bool:
				args[k] = strconv.FormatBool(vv)
			default:
				b, _ := json.Marshal(vv)
				args[k] = string(b)
			}
		}
		slog.Info("mistral tool call", "tool", acc.name, "arg_keys", keysOf(args))
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			Name: acc.name,
			Args: args,
		})
	}

	if onDelta != nil {
		onDelta(StreamDelta{Done: true, Response: result})
	}
	return result, nil
}

func (m *MistralProvider) Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	mistralMessages := toMistralMessages(messages)

	reqBody := mistralRequest{
		Model:    m.model,
		Messages: mistralMessages,
	}

	if len(tools) > 0 {
		for _, t := range tools {
			reqBody.Tools = append(reqBody.Tools, mistralTool{
				Type: "function",
				Function: mistralFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			})
		}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}

	resp, err := httputil.DoWithRetry(ctx, m.client, req, 5)
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

	var mistralResp mistralResponse
	if err := json.Unmarshal(respBody, &mistralResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if len(mistralResp.Choices) == 0 {
		return &Response{}, nil
	}

	result := &Response{
		Content: mistralResp.Choices[0].Message.Content,
		Usage: Usage{
			InputTokens:  mistralResp.Usage.PromptTokens,
			OutputTokens: mistralResp.Usage.CompletionTokens,
		},
	}
	for _, tc := range mistralResp.Choices[0].Message.ToolCalls {
		rawArgs := tc.Function.Arguments
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(rawArgs), &parsed); err != nil {
			slog.Warn("mistral tool args parse failed", "tool", tc.Function.Name, "raw", rawArgs, "err", err)
			continue
		}
		args := make(map[string]string, len(parsed))
		for k, v := range parsed {
			if v == nil {
				continue
			}
			switch vv := v.(type) {
			case string:
				args[k] = vv
			case float64:
				args[k] = strconv.FormatFloat(vv, 'f', -1, 64)
			case bool:
				args[k] = strconv.FormatBool(vv)
			default:
				b, _ := json.Marshal(vv)
				args[k] = string(b)
			}
		}
		slog.Info("mistral tool call", "tool", tc.Function.Name, "arg_keys", keysOf(args), "raw_len", len(rawArgs))
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			Name: tc.Function.Name,
			Args: args,
		})
	}

	return result, nil
}

type mistralMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type mistralFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type mistralTool struct {
	Type     string          `json:"type"`
	Function mistralFunction `json:"function"`
}

type mistralRequest struct {
	Model    string           `json:"model"`
	Messages []mistralMessage `json:"messages"`
	Tools    []mistralTool    `json:"tools,omitempty"`
	Stream   bool             `json:"stream,omitempty"`
}

type mistralStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string                   `json:"content"`
			ToolCalls []mistralStreamToolCall   `json:"tool_calls,omitempty"`
		} `json:"delta"`
	} `json:"choices"`
	Usage mistralUsage `json:"usage"`
}

type mistralStreamToolCall struct {
	Index    int `json:"index"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type mistralToolCall struct {
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type mistralUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type mistralResponse struct {
	Choices []struct {
		Message struct {
			Content   string            `json:"content"`
			ToolCalls []mistralToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Usage mistralUsage `json:"usage"`
}
