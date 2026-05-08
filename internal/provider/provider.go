package provider

import (
	"context"
	"fmt"

	"github.com/meeseeks/koko/internal/config"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

type Image struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type Message struct {
	Role    Role    `json:"role"`
	Content string  `json:"content"`
	Images  []Image `json:"images,omitempty"`
}

type ToolCall struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type Response struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Usage     Usage      `json:"usage"`
}

type StreamDelta struct {
	Text     string
	Done     bool
	Response *Response
}

type Provider interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error)
	ChatStream(ctx context.Context, messages []Message, tools []ToolDef, onDelta func(StreamDelta)) (*Response, error)
	Name() string
	Model() string
	SetModel(model string)
}

type ToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

func chatStreamFallback(ctx context.Context, p Provider, messages []Message, tools []ToolDef, onDelta func(StreamDelta)) (*Response, error) {
	resp, err := p.Chat(ctx, messages, tools)
	if err != nil {
		return nil, err
	}
	if onDelta != nil && resp.Content != "" {
		onDelta(StreamDelta{Text: resp.Content})
	}
	if onDelta != nil {
		onDelta(StreamDelta{Done: true, Response: resp})
	}
	return resp, nil
}

func sanitizeErrorBody(body []byte, maxLen int) string {
	s := string(body)
	if len(s) > maxLen {
		s = s[:maxLen] + "...(truncated)"
	}
	return s
}

func New(cfg *config.Config) (Provider, error) {
	switch cfg.Llm.Provider {
	case config.Anthropic:
		return NewAnthropic(cfg.Llm.ApiKey, cfg.Llm.Model, cfg.Llm.MaxTokens)
	case config.Mistral:
		return NewMistral(cfg.Llm.ApiKey, cfg.Llm.Model, cfg.Llm.Url)
	case config.Ollama:
		return NewOllama(cfg.Llm.Model, cfg.Llm.Url)
	default:
		return nil, fmt.Errorf("unsupported provider: %q", cfg.Llm.Provider)
	}
}
