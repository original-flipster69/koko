package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/original-flipster69/koko/internal/config"
)

type role string

const (
	User      role = "user"
	Assistant role = "assistant"
	System    role = "system"
)

type Img struct {
	Mime string `json:"mime_type"`
	Data string `json:"data"`
}

type Msg struct {
	Role    role   `json:"role"`
	Content string `json:"content"`
	Imgs    []Img  `json:"images,omitempty"`
}

type ToolCall struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args"`
}

func (tc ToolCall) ArgsFormat() string {
	args := ""
	for k, v := range tc.Args {
		if args != "" {
			args += ", "
		}
		args += "'" + k + "': \"" + v + "\""
	}
	return args
}

type Usg struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type Response struct {
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	Usage      Usg        `json:"usage"`
	StopReason string     `json:"stop_reason,omitempty"`
}

type StreamDelta struct {
	Text     string
	Done     bool
	Response *Response
}

type Provider interface {
	ChatStream(ctx context.Context, msgs []Msg, tools []ToolDef, onDelta func(StreamDelta)) (*Response, error)
	Name() string
	Model() string
	SetModel(model string)
}

type TokenCounter interface {
	CountTokens(ctx context.Context, msgs []Msg, tools []ToolDef) (int, error)
}

type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Params      Schema `json:"parameters"`
}

type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

func StringParam(desc string) Property {
	return Property{Type: "string", Description: desc}
}

func IntParam(desc string) Property {
	return Property{Type: "integer", Description: desc}
}

func BoolParam(desc string) Property {
	return Property{Type: "boolean", Description: desc}
}

func sendReq(ctx context.Context, client *http.Client, url string, body any, headers map[string]string) (*http.Response, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}
	resp, err := withRetry(ctx, client, req, 5)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, truncate(b, 512))
	}
	return resp, nil
}

func truncate(body []byte, maxLen int) string {
	s := string(body)
	if len(s) > maxLen {
		s = s[:maxLen] + "...(truncated)"
	}
	return s
}

func coerceArgs(input map[string]interface{}) map[string]string {
	args := make(map[string]string, len(input))
	for k, v := range input {
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
	return args
}

func New(cfg *config.LlmConfig) (Provider, error) {
	switch cfg.Provider {
	case config.Anthropic:
		return newClaude(cfg.ApiKey, cfg.Model, cfg.Url, cfg.MaxTokens)
	case config.Mistral:
		return newMistral(cfg.ApiKey, cfg.Model, cfg.Url, cfg.Conversations)
	case config.Ollama:
		return newOllama(cfg.Model, cfg.Url)
	default:
		return nil, fmt.Errorf("unsupported provider: %q", cfg.Provider)
	}
}
