package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type ollama struct {
	model   string
	baseURL string
	client  *http.Client
}

func newOllama(model, baseURL string) (*ollama, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3"
	}
	return &ollama{
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{},
	}, nil
}

func (o *ollama) Name() string      { return "ollama" }
func (o *ollama) Model() string     { return o.model }
func (o *ollama) SetModel(m string) { o.model = m }

func toOllamaMsgs(messages []Msg) []ollamaMsg {
	var out []ollamaMsg
	for _, m := range messages {
		msg := ollamaMsg{Role: string(m.Role), Content: m.Content}
		for _, img := range m.Imgs {
			msg.Images = append(msg.Images, img.Data)
		}
		out = append(out, msg)
	}
	return out
}

func (o *ollama) ChatStream(ctx context.Context, messages []Msg, tools []ToolDef, onDelta func(StreamDelta)) (*Response, error) {
	reqBody := ollamaReq{
		Model:    o.model,
		Messages: toOllamaMsgs(messages),
		Stream:   true,
	}
	if len(tools) > 0 {
		reqBody.Tools = toOllama(tools)
		reqBody.Stream = false
		slog.Debug("ollama: tools present, falling back to non-streaming request")
	}

	resp, err := sendReq(ctx, o.client, o.baseURL+"/api/chat", reqBody, nil)
	if err != nil {
		return nil, fmt.Errorf("ollama (is it running?): %w", err)
	}
	defer resp.Body.Close()

	if len(tools) > 0 {
		return o.parseNonStreamingResponse(resp.Body, onDelta)
	}
	return o.parseStreamingResponse(resp.Body, onDelta)
}

func (o *ollama) parseStreamingResponse(body io.Reader, onDelta func(StreamDelta)) (*Response, error) {
	var content strings.Builder
	var usage Usg
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var chunk ollamaResp
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			content.WriteString(chunk.Message.Content)
			if onDelta != nil {
				onDelta(StreamDelta{Text: chunk.Message.Content})
			}
		}
		if chunk.Done {
			usage = Usg{
				InputTokens:  chunk.PromptEvalCount,
				OutputTokens: chunk.EvalCount,
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading stream: %w", err)
	}

	result := &Response{Content: content.String(), Usage: usage}
	if onDelta != nil {
		onDelta(StreamDelta{Done: true, Response: result})
	}
	return result, nil
}

func (o *ollama) parseNonStreamingResponse(body io.Reader, onDelta func(StreamDelta)) (*Response, error) {
	respBody, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var ollamaResp ollamaResp
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	result := &Response{
		Content: ollamaResp.Message.Content,
		Usage: Usg{
			InputTokens:  ollamaResp.PromptEvalCount,
			OutputTokens: ollamaResp.EvalCount,
		},
	}

	if ollamaResp.Message.Content != "" && onDelta != nil {
		onDelta(StreamDelta{Text: ollamaResp.Message.Content})
	}

	for _, tc := range ollamaResp.Message.ToolCalls {
		args := coerceArgs(tc.Function.Arguments)
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			Name: tc.Function.Name,
			Args: args,
		})
	}

	if onDelta != nil {
		onDelta(StreamDelta{Done: true, Response: result})
	}
	return result, nil
}

func toOllama(tools []ToolDef) []ollamaTool {
	out := make([]ollamaTool, len(tools))
	for i, t := range tools {
		out[i] = ollamaTool{
			Type: "function",
			Function: ollamaFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Params,
			},
		}
	}
	return out
}

type ollamaMsg struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Images    []string         `json:"images,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaFuncCall `json:"function"`
}

type ollamaFuncCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ollamaTool struct {
	Type     string     `json:"type"`
	Function ollamaFunc `json:"function"`
}

type ollamaFunc struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type ollamaReq struct {
	Model    string       `json:"model"`
	Messages []ollamaMsg  `json:"messages"`
	Stream   bool         `json:"stream"`
	Tools    []ollamaTool `json:"tools,omitempty"`
}

type ollamaResp struct {
	Message         ollamaMsg `json:"message"`
	Done            bool      `json:"done"`
	PromptEvalCount int       `json:"prompt_eval_count"`
	EvalCount       int       `json:"eval_count"`
}
